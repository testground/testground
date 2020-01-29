package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ api.Runner = &ClusterK8sRunner{}
)

var k8sSubnetIdx uint64 = 0

func init() {
	// Avoid collisions in picking up subnets
	rand.Seed(time.Now().UnixNano())
	k8sSubnetIdx = rand.Uint64() % 4096
}

func nextK8sSubnet() (*net.IPNet, error) {
	subnet, _, err := nextDataNetwork(int(atomic.AddUint64(&k8sSubnetIdx, 1) % 4096))
	if err != nil {
		return nil, err
	}
	_, n, err := net.ParseCIDR(subnet)
	return n, err
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// ClusterK8sRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type ClusterK8sRunnerConfig struct {
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`

	// Background avoids tailing the output of containers, and displaying it as
	// log messages (default: true).
	Background bool `toml:"background"`

	KeepService bool `toml:"keep_service"`
}

// ClusterK8sRunner is a runner that creates a Docker service to launch as
// many replicated instances of a container as the run job indicates.
type ClusterK8sRunner struct{}

type KubernetesConfig struct {
	// KubeConfigPath is the path to your kubernetes configuration path
	KubeConfigPath string `json:"kubeConfigPath"`
	// Namespace is the kubernetes namespaces where the pods should be running
	Namespace string `json:"namespace"`
}

// defaultKubernetesConfig uses the default ~/.kube/config
// to discover the kubernetes clusters. It also uses the "default" namespace.
func defaultKubernetesConfig() KubernetesConfig {
	kubeconfig := filepath.Join(homeDir(), ".kube", "config")
	return KubernetesConfig{
		KubeConfigPath: kubeconfig,
		Namespace:      "default",
	}
}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (*ClusterK8sRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		image = input.ArtifactPath
		seq   = input.Seq
		log   = logging.S().With("runner", "cluster:k8s", "run_id", input.RunID)
		cfg   = *input.RunnerConfig.(*ClusterK8sRunnerConfig)
	)

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	testcase := input.TestPlan.TestCases[seq]

	// Build a runenv.
	runenv := &runtime.RunEnv{
		TestPlan:           input.TestPlan.Name,
		TestCase:           testcase.Name,
		TestRun:            input.RunID,
		TestCaseSeq:        seq,
		TestInstanceCount:  input.Instances,
		TestInstanceParams: input.Parameters,
		TestSidecar:        true,
		TestArtifacts:      "/artifacts",
	}

	// currently weave is not releaasing IP addresses upon container deletion - we get errors back when trying to
	// use an already used IP address, even if the container has been removed
	// this functionality should be refactored asap, when we understand how weave releases IPs (or why it doesn't release
	// them when a container is removed/ and as soon as we decide how to manage `networks in-use` so that there are no
	// collisions in concurrent testplan runs
	var err error
	runenv.TestSubnet, err = nextK8sSubnet()
	if err != nil {
		return nil, err
	}

	env := util.ToEnvVar(runenv.ToEnvVars())

	// Set the log level if provided in cfg.
	if cfg.LogLevel != "" {
		env = append(env, v1.EnvVar{
			Name:  "LOG_LEVEL",
			Value: cfg.LogLevel,
		})
	}

	redisCfg := v1.EnvVar{
		Name:  "REDIS_HOST",
		Value: "redis-headless",
	}

	env = append(env, redisCfg)

	// Define k8s client configuration
	config := defaultKubernetesConfig()

	workers := 20
	pool, err := NewPool(workers, config)
	if err != nil {
		return nil, err
	}

	var (
		replicas = uint64(input.Instances)
	)

	jobName := fmt.Sprintf("tg-%s", input.TestPlan.Name)

	log.Infow("deploying testground testplan run on k8s", "job-name", jobName, "image", image, "replicas", replicas)

	mode := v1.MountPropagationHostToContainer
	hostpathtype := v1.HostPathType("DirectoryOrCreate")

	var g errgroup.Group

	g.Go(func() error {
		client, err := pool.Get(ctx)
		if err != nil {
			return err
		}
		defer pool.Put(client)

		start := time.Now()
		for {
			if time.Since(start) > 10*time.Minute {
				return errors.New("global timeout")
			}
			time.Sleep(5000 * time.Millisecond)

			list := func(state string) int {
				fieldSelector := fmt.Sprintf("status.phase=%s", state)
				opts := metav1.ListOptions{
					LabelSelector: fmt.Sprintf("testground.runid=%s", input.RunID),
					FieldSelector: fieldSelector,
				}
				res, err := client.CoreV1().Pods(config.Namespace).List(opts)
				if err != nil {
					log.Warnw("k8s client pods list error", "err", err.Error())
					return -1
				}
				return len(res.Items)
			}

			results := map[string]int{}
			var resultsMu sync.Mutex
			states := []string{"Pending", "Running", "Succeeded", "Failed", "Unknown"}

			var wg sync.WaitGroup
			wg.Add(len(states))
			for _, state := range states {
				state := state
				go func() {
					defer wg.Done()

					result := list(state)

					resultsMu.Lock()
					results[state] = result
					resultsMu.Unlock()
				}()
			}
			wg.Wait()

			log.Debugw("testplan state", "succeeded", results["Succeeded"], "running", results["Running"], "pending", results["Pending"], "failed", results["Failed"], "unknown", results["Unknown"])

			if results["Succeeded"] == int(replicas) {
				return nil
			}
		}

		return nil
	})

	sem := make(chan struct{}, 30) // limit the number of concurrent k8s api calls

	for i := uint64(1); i <= replicas; i++ {
		i := i
		sem <- struct{}{}

		podName := fmt.Sprintf("%s-%d", jobName, i)

		defer func() {
			if cfg.KeepService {
				return
			}
			client, err := pool.Get(ctx)
			if err != nil {
				log.Errorw("couldn't get client from pool", "pod", podName, "err", err)
			}
			defer pool.Put(client)
			err = client.CoreV1().Pods("default").Delete(podName, &metav1.DeleteOptions{})
			if err != nil {
				log.Errorw("couldn't remove pod", "pod", podName, "err", err)
			}
		}()

		g.Go(func() error {
			defer func() { <-sem }()

			client, err := pool.Get(ctx)
			if err != nil {
				return err
			}
			defer pool.Put(client)

			mnt := v1.HostPathVolumeSource{
				Path: fmt.Sprintf("/mnt/%s/%s", input.RunID, podName),
				Type: &hostpathtype,
			}

			// Create Kubernetes Pod
			podRequest := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
					Labels: map[string]string{
						"testground.plan":     input.TestPlan.Name,
						"testground.testcase": testcase.Name,
						"testground.runid":    input.RunID,
					},
					Annotations: map[string]string{"cni": "flannel"},
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name:         "s3-shared",
							VolumeSource: v1.VolumeSource{HostPath: &mnt},
						},
					},
					SecurityContext: &v1.PodSecurityContext{
						Sysctls: []v1.Sysctl{{Name: "net.core.somaxconn", Value: "10000"}},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:  podName,
							Image: image,
							Args:  []string{},
							Env:   env,
							VolumeMounts: []v1.VolumeMount{
								{
									Name:             "s3-shared",
									MountPath:        runenv.TestArtifacts,
									MountPropagation: &mode,
								},
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceMemory: resource.MustParse("100Mi"),
									v1.ResourceCPU:    resource.MustParse("100m"),
								},
							},
						},
					},
				},
			}

			_, err = client.CoreV1().Pods(config.Namespace).Create(podRequest)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	for i := uint64(1); i <= replicas; i++ {
		i := i
		sem <- struct{}{}

		g.Go(func() error {
			defer func() { <-sem }()

			client, err := pool.Get(ctx)
			if err != nil {
				return err
			}
			defer pool.Put(client)

			podName := fmt.Sprintf("%s-%d", jobName, i)

			logs := getPodLogs(client, podName)

			fmt.Print(logs)
			return nil
		})
	}

	out := &api.RunOutput{RunnerID: "smth"}
	return out, nil
}

func (*ClusterK8sRunner) ID() string {
	return "cluster:k8s"
}

func (*ClusterK8sRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(ClusterK8sRunnerConfig{})
}

func (*ClusterK8sRunner) CompatibleBuilders() []string {
	return []string{"docker:go"}
}

func int32Ptr(i int32) *int32 { return &i }

func int64Ptr(i int64) *int64 { return &i }

func getPodLogs(clientset *kubernetes.Clientset, podName string) string {
	podLogOpts := v1.PodLogOptions{
		TailLines: int64Ptr(2),
	}
	req := clientset.CoreV1().Pods("default").GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream()
	if err != nil {
		return "error in opening stream"
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from podLogs to buf"
	}
	str := buf.String()

	return str
}

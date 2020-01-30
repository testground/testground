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
	"go.uber.org/zap"
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

const defaultK8sNetworkAnnotation = "flannel"

var (
	testplanSysctls = []v1.Sysctl{{Name: "net.core.somaxconn", Value: "10000"}}
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

func (*ClusterK8sRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		log = logging.S().With("runner", "cluster:k8s", "run_id", input.RunID)
		cfg = *input.RunnerConfig.(*ClusterK8sRunnerConfig)
	)

	// Sanity check.
	if input.Seq < 0 || input.Seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", input.Seq, input.TestPlan.Name)
	}

	// Build a runenv.
	runenv := &runtime.RunEnv{
		TestPlan:           input.TestPlan.Name,
		TestCase:           input.TestPlan.TestCases[input.Seq].Name,
		TestRun:            input.RunID,
		TestCaseSeq:        input.Seq,
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

	k8sConfig := defaultKubernetesConfig()

	workers := 20
	pool, err := newPool(workers, k8sConfig)
	if err != nil {
		return nil, err
	}

	jobName := fmt.Sprintf("tg-%s", input.TestPlan.Name)

	log.Infow("deploying testground testplan run on k8s", "job-name", jobName, "image", input.ArtifactPath, "replicas", input.Instances)

	var g errgroup.Group

	replicas := input.Instances

	g.Go(func() error {
		return monitorTestplanRunState(ctx, pool, log, input.RunID, k8sConfig.Namespace, int(input.Instances))
	})

	sem := make(chan struct{}, 30) // limit the number of concurrent k8s api calls

	for i := 1; i <= replicas; i++ {
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

			return createPod(ctx, pool, podName, input, runenv, env, k8sConfig.Namespace)
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	for i := 1; i <= replicas; i++ {
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

	return &api.RunOutput{}, nil
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

func monitorTestplanRunState(ctx context.Context, pool *pool, log *zap.SugaredLogger, runID string, k8sNamespace string, replicas int) error {
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
		time.Sleep(2000 * time.Millisecond)

		countPodsByState := func(state string) int {
			fieldSelector := fmt.Sprintf("status.phase=%s", state)
			opts := metav1.ListOptions{
				LabelSelector: fmt.Sprintf("testground.runid=%s", runID),
				FieldSelector: fieldSelector,
			}
			res, err := client.CoreV1().Pods(k8sNamespace).List(opts)
			if err != nil {
				log.Warnw("k8s client pods list error", "err", err.Error())
				return -1
			}
			return len(res.Items)
		}

		counters := map[string]int{}
		var countersMu sync.Mutex
		states := []string{"Pending", "Running", "Succeeded", "Failed", "Unknown"}

		var wg sync.WaitGroup
		wg.Add(len(states))
		for _, state := range states {
			state := state
			go func() {
				defer wg.Done()

				n := countPodsByState(state)

				countersMu.Lock()
				counters[state] = n
				countersMu.Unlock()
			}()
		}
		wg.Wait()

		log.Debugw("testplan state", "succeeded", counters["Succeeded"], "running", counters["Running"], "pending", counters["Pending"], "failed", counters["Failed"], "unknown", counters["Unknown"])

		if counters["Succeeded"] == replicas {
			return nil
		}
	}
}

func createPod(ctx context.Context, pool *pool, podName string, input *api.RunInput, runenv *runtime.RunEnv, env []v1.EnvVar, k8sNamespace string) error {
	client, err := pool.Get(ctx)
	if err != nil {
		return err
	}
	defer pool.Put(client)

	mountPropagationMode := v1.MountPropagationHostToContainer
	hostpathtype := v1.HostPathType("DirectoryOrCreate")
	sharedVolumeName := "s3-shared"

	mnt := v1.HostPathVolumeSource{
		Path: fmt.Sprintf("/mnt/%s/%s", input.RunID, podName),
		Type: &hostpathtype,
	}

	podRequest := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"testground.plan":     input.TestPlan.Name,
				"testground.testcase": runenv.TestCase,
				"testground.runid":    input.RunID,
			},
			Annotations: map[string]string{"cni": defaultK8sNetworkAnnotation},
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name:         sharedVolumeName,
					VolumeSource: v1.VolumeSource{HostPath: &mnt},
				},
			},
			SecurityContext: &v1.PodSecurityContext{
				Sysctls: testplanSysctls,
			},
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:  podName,
					Image: input.ArtifactPath,
					Args:  []string{},
					Env:   env,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             sharedVolumeName,
							MountPath:        runenv.TestArtifacts,
							MountPropagation: &mountPropagationMode,
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

	_, err = client.CoreV1().Pods(k8sNamespace).Create(podRequest)
	return err
}

func int64Ptr(i int64) *int64 { return &i }

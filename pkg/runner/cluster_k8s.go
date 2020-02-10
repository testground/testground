package runner

import (
	"archive/zip"
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

	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"go.uber.org/zap"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ api.Runner = &ClusterK8sRunner{}
)

const (
	defaultK8sNetworkAnnotation = "flannel"
)

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
	return subnet, err
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

	KeepService bool `toml:"keep_service"`

	// Name of the S3 bucket used for `outputs` from test plans
	OutputsBucket string `toml:"outputs_bucket"`

	// Region of the S3 bucket used for `outputs` from test plans
	OutputsBucketRegion string `toml:"outputs_bucket_region"`
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

	template := runtime.RunParams{
		TestPlan:          input.TestPlan.Name,
		TestCase:          input.TestPlan.TestCases[input.Seq].Name,
		TestRun:           input.RunID,
		TestCaseSeq:       input.Seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
		TestOutputsPath:   "/outputs",
	}

	// currently weave is not releaasing IP addresses upon container deletion - we get errors back when trying to
	// use an already used IP address, even if the container has been removed
	// this functionality should be refactored asap, when we understand how weave releases IPs (or why it doesn't release
	// them when a container is removed/ and as soon as we decide how to manage `networks in-use` so that there are no
	// collisions in concurrent testplan runs
	subnet, err := nextK8sSubnet()
	if err != nil {
		return nil, err
	}

	template.TestSubnet = &runtime.IPNet{IPNet: *subnet}

	k8sConfig := defaultKubernetesConfig()

	workers := 20
	pool, err := newPool(workers, k8sConfig)
	if err != nil {
		return nil, err
	}

	jobName := fmt.Sprintf("tg-%s", input.TestPlan.Name)

	log.Infow("deploying testground testplan run on k8s", "job-name", jobName)

	var eg errgroup.Group

	eg.Go(func() error {
		return monitorTestplanRunState(ctx, pool, log, input, k8sConfig.Namespace)
	})

	sem := make(chan struct{}, 30) // limit the number of concurrent k8s api calls

	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupID = g.ID
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestInstanceParams = g.Parameters

		env := conv.ToEnvVar(runenv.ToEnvVars())
		env = append(env, v1.EnvVar{
			Name:  "REDIS_HOST",
			Value: "redis-headless",
		})

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, v1.EnvVar{
				Name:  "LOG_LEVEL",
				Value: cfg.LogLevel,
			})
		}
		for i := 0; i < g.Instances; i++ {
			i := i
			sem <- struct{}{}

			podName := fmt.Sprintf("%s-%s-%s-%d", jobName, input.RunID, g.ID, i)

			defer func() {
				if cfg.KeepService {
					return
				}
				client := pool.Acquire()
				defer pool.Release(client)
				err = client.CoreV1().Pods("default").Delete(podName, &metav1.DeleteOptions{})
				if err != nil {
					log.Errorw("couldn't remove pod", "pod", podName, "err", err)
				}
			}()

			eg.Go(func() error {
				defer func() { <-sem }()

				return createPod(ctx, pool, podName, input, runenv, env, k8sConfig.Namespace, g, i)
			})
		}
	}

	err = eg.Wait()
	if err != nil {
		return nil, err
	}

	var gg errgroup.Group

	for _, g := range input.Groups {

		for i := 0; i < g.Instances; i++ {
			i := i
			sem <- struct{}{}

			gg.Go(func() error {
				defer func() { <-sem }()

				client := pool.Acquire()
				defer pool.Release(client)

				podName := fmt.Sprintf("%s-%s-%s-%d", jobName, input.RunID, g.ID, i)

				logs := getPodLogs(client, podName)

				fmt.Print(logs)
				return nil
			})
		}
	}

	err = gg.Wait()
	if err != nil {
		return nil, err
	}

	return &api.RunOutput{RunID: input.RunID}, nil
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

func (*ClusterK8sRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, w io.Writer) error {
	log := logging.S().With("runner", "cluster:k8s", "run_id", input.RunID)
	cfg := *input.RunnerConfig.(*ClusterK8sRunnerConfig)

	log.Info("collecting outputs")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.OutputsBucketRegion)},
	)
	if err != nil {
		return fmt.Errorf("Couldn't establish an AWS session to list items in bucket: %v", err)
	}

	svc := s3.New(sess)

	startAfter := ""
	downloader := s3manager.NewDownloader(sess)

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for {
		log.Debugw("start after", "cursor", startAfter)
		resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{StartAfter: &startAfter, Bucket: aws.String(cfg.OutputsBucket), Prefix: aws.String(input.RunID)})
		if err != nil {
			return fmt.Errorf("Unable to list items in bucket %q, %v", cfg.OutputsBucket, err)
		}

		if len(resp.Contents) == 0 {
			break
		}

		log.Debugw("got contents", "len", len(resp.Contents))
		for _, item := range resp.Contents {
			ww, err := zipWriter.Create(*item.Key)
			if err != nil {
				return fmt.Errorf("Couldn't add file to the zip archive: %v", err)
			}

			_, err = downloader.Download(FakeWriterAt{ww},
				&s3.GetObjectInput{
					Bucket: aws.String(cfg.OutputsBucket),
					Key:    item.Key,
				})
			if err != nil {
				return fmt.Errorf("Couldn't download item from S3: %q, err: %v", item, err)
			}
		}
		startAfter = *(resp.Contents[len(resp.Contents)-1].Key)
	}

	return nil
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

func monitorTestplanRunState(ctx context.Context, pool *pool, log *zap.SugaredLogger, input *api.RunInput, k8sNamespace string) error {
	client := pool.Acquire()
	defer pool.Release(client)

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Since(start) > 10*time.Minute {
			return errors.New("global timeout")
		}
		time.Sleep(2000 * time.Millisecond)

		countPodsByState := func(state string) int {
			fieldSelector := fmt.Sprintf("status.phase=%s", state)
			opts := metav1.ListOptions{
				LabelSelector: fmt.Sprintf("testground.run_id=%s", input.RunID),
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

		if counters["Succeeded"] == input.TotalInstances {
			return nil
		}
	}
}

func createPod(ctx context.Context, pool *pool, podName string, input *api.RunInput, runenv runtime.RunParams, env []v1.EnvVar, k8sNamespace string, g api.RunGroup, i int) error {
	client := pool.Acquire()
	defer pool.Release(client)

	mountPropagationMode := v1.MountPropagationHostToContainer
	hostpathtype := v1.HostPathType("DirectoryOrCreate")
	sharedVolumeName := "s3-shared"

	mnt := v1.HostPathVolumeSource{
		Path: fmt.Sprintf("/mnt/%s/%s/%d", input.RunID, g.ID, i),
		Type: &hostpathtype,
	}

	podRequest := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"testground.plan":     input.TestPlan.Name,
				"testground.testcase": runenv.TestCase,
				"testground.run_id":   input.RunID,
				"testground.groupid":  g.ID,
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
					Image: g.ArtifactPath,
					Args:  []string{},
					Env:   env,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             sharedVolumeName,
							MountPath:        runenv.TestOutputsPath,
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

	_, err := client.CoreV1().Pods(k8sNamespace).Create(podRequest)
	return err
}

func int64Ptr(i int64) *int64 { return &i }

type FakeWriterAt struct {
	w io.Writer
}

func (fw FakeWriterAt) WriteAt(p []byte, offset int64) (n int, err error) {
	// ignore 'offset' because we forced sequential downloads
	return fw.w.Write(p)
}

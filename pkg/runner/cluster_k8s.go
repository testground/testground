package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	v1batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ api.Runner = &ClusterK8sRunner{}
)

func init() {
	// Avoid collisions in picking up subnets
	rand.Seed(time.Now().UnixNano())
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
		seq = input.Seq
		log = logging.S().With("runner", "cluster:k8s", "run_id", input.RunID)
		cfg = *input.RunnerConfig.(*ClusterK8sRunnerConfig)
	)

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	testcase := input.TestPlan.TestCases[seq]

	// Build a runenv.
	template := runtime.RunEnv{
		TestPlan:          input.TestPlan.Name,
		TestCase:          testcase.Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
	}

	// currently weave is not releaasing IP addresses upon container deletion - we get errors back when trying to
	// use an already used IP address, even if the container has been removed
	// this functionality should be refactored asap, when we understand how weave releases IPs (or why it doesn't release
	// them when a container is removed/ and as soon as we decide how to manage `networks in-use` so that there are no
	// collisions in concurrent testplan runs
	var err error
	b := 1 + rand.Intn(200)
	_, template.TestSubnet, err = net.ParseCIDR(fmt.Sprintf("10.%d.0.0/16", b))
	if err != nil {
		return nil, err
	}

	// Define k8s client configuration
	config := defaultKubernetesConfig()
	k8scfg, err := clientcmd.BuildConfigFromFlags("", config.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not start k8s client from config: %v", err)
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(k8scfg)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}

	var (
		parent = fmt.Sprintf("tg-%s-%s-%s", input.TestPlan.Name, testcase.Name, input.RunID)
		jobs   []string
	)

	defer func() {
		log.Debugw("configuration for job", "keep_service", cfg.KeepService)
		if cfg.KeepService {
			log.Info("skipping removing jobs due to user request")
			return
		}
		for _, j := range jobs {
			err := clientset.BatchV1().Jobs(config.Namespace).Delete(j, &metav1.DeleteOptions{})
			if err != nil {
				log.Errorw("couldn't remove job", "job", j, "err", err)
			}
		}
	}()

	errgrp, errctx := errgroup.WithContext(ctx)
	for _, g := range input.Groups {
		jobName := strings.Join([]string{parent, g.ID}, ":")

		jobs = append(jobs, jobName)

		log.Infow("creating k8s deployment", "job", jobName, "image", g.ArtifactPath, "instances", g.Instances)

		runenv := template
		runenv.TestGroupID = g.ID
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestInstanceParams = g.Parameters

		env := conv.ToEnvVar(runenv.ToEnvVars())
		redisCfg := v1.EnvVar{
			Name:  "REDIS_HOST",
			Value: "redis-headless",
		}
		env = append(env, redisCfg)

		// Create Kubernetes Job
		jobRequest := &v1batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
				Labels: map[string]string{
					"testground.plan":     input.TestPlan.Name,
					"testground.testcase": testcase.Name,
					"testground.runid":    input.RunID,
					"testground.groupid":  g.ID,
				},
			},
			Spec: v1batch.JobSpec{
				Parallelism:             int32Ptr(int32(g.Instances)),
				Completions:             int32Ptr(int32(g.Instances)),
				TTLSecondsAfterFinished: int32Ptr(600),
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"testground.plan":     input.TestPlan.Name,
							"testground.testcase": testcase.Name,
							"testground.runid":    input.RunID,
							"testground.groupid":  g.ID,
						},
						Annotations: map[string]string{
							"cni": "flannel",
						},
					},
					Spec: v1.PodSpec{
						RestartPolicy: v1.RestartPolicyNever,
						Containers: []v1.Container{
							{
								Name:  jobName,
								Image: g.ArtifactPath,
								Args:  []string{},
								Env:   env,
								Resources: v1.ResourceRequirements{
									Limits: v1.ResourceList{
										v1.ResourceMemory: resource.MustParse("100Mi"),
										v1.ResourceCPU:    resource.MustParse("100m"),
									},
								},
							},
						},
					},
				},
			},
		}

		errgrp.Go(func() error {
			_, err = clientset.BatchV1().Jobs(config.Namespace).Create(jobRequest)
			if err != nil {
				return err
			}

			// Wait for job.
			for start := time.Now(); time.Since(start) <= 5*time.Minute; {
				select {
				case <-errctx.Done():
					return errctx.Err()
				default:
				}

				job, err := clientset.BatchV1().Jobs(config.Namespace).Get(jobName, metav1.GetOptions{})
				if err != nil {
					log.Warnw("transient job error", "job", jobName, "err", err)
					time.Sleep(300 * time.Millisecond)
					continue
				}

				log.Debugw("job status", "job", jobName, "active", job.Status.Active, "succeeded", job.Status.Succeeded, "failed", job.Status.Failed)
				if job.Status.Active == 0 && (job.Status.Succeeded > 0 || job.Status.Failed > 0) {
					return nil
				}
				time.Sleep(2000 * time.Millisecond)
			}
			return errors.New("timeout")
		})
	}

	return &api.RunOutput{}, errgrp.Wait()
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

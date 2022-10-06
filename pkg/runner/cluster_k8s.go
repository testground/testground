package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/testground/sdk-go/ptypes"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/testground/sdk-go/runtime"
	ss "github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/aws"
	"github.com/testground/testground/pkg/conv"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"
	"golang.org/x/sync/errgroup"

	v1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	lru "github.com/hashicorp/golang-lru"
	"github.com/msoap/byline"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	_             api.Runner        = (*ClusterK8sRunner)(nil)
	_             api.Terminatable  = (*ClusterK8sRunner)(nil)
	_             api.Healthchecker = (*ClusterK8sRunner)(nil)
	mu                              = sync.Mutex{}
	errSyncClient                   = errors.New("failed to start sync client")
)

const (
	defaultK8sNetworkAnnotation = "aws-cni"
	// collect-outputs pod is used to compress outputs at the end of a testplan run
	// as well as to copy archives from it, since it has EFS attached to it
	collectOutputsPodName = "collect-outputs"

	// number of CPUs allocated to each Sidecar. should be same as what is set in sidecar.yaml
	sidecarCPUs = 0.2

	// utilisation is how many CPUs from the remainder shall we allocate to Testground
	// note that there are other services running on the Kubernetes cluster such as
	// api proxy, node_exporter, dummy, etc.
	utilisation = 0.85

	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check sdk/sync for more information
	NetworkInitialisationSuccessful = "network initialisation successful"
	NetworkInitialisationFailed     = "network initialisation failed"
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

	// Provider is the infrastructure provider to use
	Provider string `toml:"provider"`

	// Whether Kubernetes cluster has an autoscaler running
	AutoscalerEnabled bool `toml:"autoscaler_enabled"`

	// Resources requested for each testplan pod from the Kubernetes cluster
	TestplanPodMemory string `toml:"testplan_pod_memory"`
	TestplanPodCPU    string `toml:"testplan_pod_cpu"`

	// Resources requested for the `collect-outputs` pod from the Kubernetes cluster
	CollectOutputsPodMemory string `toml:"collect_outputs_pod_memory"`
	CollectOutputsPodCPU    string `toml:"collect_outputs_pod_cpu"`

	ExposedPorts ExposedPorts `toml:"exposed_ports"`

	RunTimeoutMin int `toml:"run_timeout_min"`

	Sysctls []string `toml:"sysctls"`
}

// ClusterK8sRunner is a runner that creates a Docker service to launch as
// many replicated instances of a container as the run job indicates.
type ClusterK8sRunner struct {
	initialized bool
	config      KubernetesConfig
	pool        *pool
	imagesLRU   *lru.Cache
	syncClient  *ss.DefaultClient
}

type Journal struct {
	Events       map[string]string   `json:"events"`
	PodsStatuses map[string]struct{} `json:"pods_statuses"`
}

func (r *Result) String() string {
	return fmt.Sprintf("outcome = %s (%s)", r.Outcome, r.StringOutcomes())
}

func (r *Result) StringOutcomes() string {
	groups := fmt.Sprintf("%v", r.Outcomes) // map[k:v, x:y]
	return groups[4 : len(groups)-1]        // remove the `map[` and `]` parts
}

type GroupOutcome struct {
	Ok    int `json:"ok"`
	Total int `json:"total"`
}

func (g *GroupOutcome) String() string {
	return fmt.Sprintf("%d/%d", g.Ok, g.Total)
}

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
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		kubeconfig = ""
	}
	return KubernetesConfig{
		KubeConfigPath: kubeconfig,
		Namespace:      "default",
	}
}

func (c *ClusterK8sRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (runoutput *api.RunOutput, runerr error) {
	if err := c.initPool(); err != nil {
		return nil, fmt.Errorf("could not init pool: %w", err)
	}

	result := newResult(input)
	runoutput = &api.RunOutput{
		RunID:  input.RunID,
		Result: result,
	}

	defer func() {
		if ctx.Err() == context.Canceled {
			result.Outcome = task.OutcomeCanceled
		}
	}()

	ow = ow.With("runner", "cluster:k8s", "run_id", input.RunID)

	cfg := *input.RunnerConfig.(*ClusterK8sRunnerConfig)

	// if `provider` is set, we have to push to a docker registry
	if cfg.Provider != "" {
		err := c.pushImagesToDockerRegistry(ctx, ow, input)
		if err != nil {
			runerr = fmt.Errorf("failed to push images to %s; err: %w", cfg.Provider, err)
			return
		}
	}

	defaultCPU, err := resource.ParseQuantity(cfg.TestplanPodCPU)
	if err != nil {
		runerr = fmt.Errorf("couldn't parse default test plan pod CPU request; make sure you have specified `testplan_pod_cpu` in .env.toml; err: %w", err)
		return
	}

	defaultMemory, err := resource.ParseQuantity(cfg.TestplanPodMemory)
	if err != nil {
		runerr = fmt.Errorf("couldn't parse default test plan pod Memory request; make sure you have specified `testplan_pod_memory` in .env.toml; err: %w", err)
		return
	}

	template := runtime.RunParams{
		TestPlan:           input.TestPlan,
		TestCase:           input.TestCase,
		TestRun:            input.RunID,
		TestInstanceCount:  input.TotalInstances,
		TestDisableMetrics: input.DisableMetrics,
		TestSidecar:        true,
		TestOutputsPath:    "/outputs",
		TestStartTime:      time.Now(),
	}

	// currently weave is not releaasing IP addresses upon container deletion - we get errors back when trying to
	// use an already used IP address, even if the container has been removed
	// this functionality should be refactored asap, when we understand how weave releases IPs (or why it doesn't release
	// them when a container is removed/ and as soon as we decide how to manage `networks in-use` so that there are no
	// collisions in concurrent testplan runs
	subnet, err := nextK8sSubnet()
	if err != nil {
		runerr = err
		return
	}

	template.TestSubnet = &ptypes.IPNet{IPNet: *subnet}

	enoughResources, err := c.checkClusterResources(ow, input.Groups, defaultMemory, defaultCPU)
	if err != nil {
		runerr = fmt.Errorf("couldn't check cluster resources: %v", err)
		return
	}

	if !enoughResources {
		if cfg.AutoscalerEnabled {
			ow.Warnw("too many test instances requested, will have to wait for cluster autoscaler to kick in")
		} else {
			runerr = errors.New("too many test instances requested, resize cluster if you need more capacity")
			return
		}
	}

	jobName := fmt.Sprintf("tg-%s", input.TestPlan)

	ow.Infow("deploying testground testplan run on k8s", "job-name", jobName)

	var eg errgroup.Group

	eg.Go(func() error {
		ctxContainers, cancel := context.WithCancel(ctx)
		defer cancel()

		outcomesDoneCh, err := c.collectOutcomes(ctxContainers, result, &template)
		if err != nil {
			ow.Errorw("could not start collecting outcomes", "err", err)
		}

		err = c.watchRunPods(ctx, ow, input, result, &template)
		if err != nil {
			return err
		}

		cancel()
		<-outcomesDoneCh
		return nil
	})

	sem := make(chan struct{}, 30) // limit the number of concurrent k8s api calls

	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupID = g.ID
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestInstanceParams = g.Parameters
		runenv.TestCaptureProfiles = g.Profiles

		result.Outcomes[g.ID] = &GroupOutcome{
			Total: g.Instances,
		}

		env := conv.ToEnvVar(runenv.ToEnvVars())
		env = append(env, v1.EnvVar{Name: "REDIS_HOST", Value: "testground-infra-redis"})
		env = append(env, v1.EnvVar{Name: "SYNC_SERVICE_HOST", Value: "testground-sync-service"})
		env = append(env, v1.EnvVar{Name: "INFLUXDB_URL", Value: "http://influxdb:8086"})
		// This subnet should correspond to the secondary CNI's IP range (usually Weave)
		env = append(env, v1.EnvVar{Name: "TEST_SUBNET", Value: "10.32.0.0/12"})

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, v1.EnvVar{Name: "LOG_LEVEL", Value: cfg.LogLevel})
		}

		env = append(env, v1.EnvVar{Name: "POD_IP", ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP"}}})
		env = append(env, v1.EnvVar{Name: "HOST_IP", ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.hostIP"}}})

		// Inject exposed ports.
		for name, value := range cfg.ExposedPorts.ToEnvVars() {
			env = append(env, v1.EnvVar{Name: name, Value: value})
		}

		podCPU := defaultCPU
		if g.Resources.CPU != "" {
			var err error
			podCPU, err = resource.ParseQuantity(g.Resources.CPU)
			if err != nil {
				runerr = err
				return
			}
		}

		podMemory := defaultMemory
		if g.Resources.Memory != "" {
			var err error
			podMemory, err = resource.ParseQuantity(g.Resources.Memory)
			if err != nil {
				runerr = err
				return
			}
		}

		for i := 0; i < g.Instances; i++ {
			i := i
			g := g
			sem <- struct{}{}

			podName := fmt.Sprintf("%s-%s-%s-%d", jobName, input.RunID, g.ID, i)

			defer func() {
				if cfg.KeepService {
					return
				}
				client := c.pool.Acquire()
				defer c.pool.Release(client)
				ow.Debugw("deleting pod", "pod", podName)
				err = client.CoreV1().Pods(c.config.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
				if err != nil {
					ow.Errorw("couldn't remove pod", "pod", podName, "err", err)
				}
			}()

			eg.Go(func() error {
				defer func() { <-sem }()

				currentEnv := make([]v1.EnvVar, len(env))
				copy(currentEnv, env)

				currentEnv = append(currentEnv, v1.EnvVar{
					Name:  "TEST_OUTPUTS_PATH",
					Value: fmt.Sprintf("/outputs/%s/%s/%d", input.RunID, g.ID, i),
				})

				return c.createTestplanPod(ctx, podName, input, runenv, currentEnv, g, i, podMemory, podCPU)
			})
		}
	}

	// we want to fetch logs even in an event of error
	defer func() {
		if input.TotalInstances <= 200 {
			var gg errgroup.Group

			for _, g := range input.Groups {
				for i := 0; i < g.Instances; i++ {
					i := i
					g := g
					sem <- struct{}{}

					gg.Go(func() error {
						defer func() { <-sem }()

						podName := fmt.Sprintf("%s-%s-%s-%d", jobName, input.RunID, g.ID, i)

						ow.Debugw("fetching logs", "pod", podName)
						logs, err := c.getPodLogs(ow, podName)
						if err != nil {
							return err
						}
						ow.Debugw("got logs", "pod", podName, "len", len(logs))

						_, err = ow.WriteProgress([]byte(logs))
						return err
					})
				}
			}

			err = gg.Wait()
			if err != nil {
				ow.Errorw("error while fetching logs", "err", err.Error())
			}

			ow.Debugw("done getting logs")
		}
	}()

	err = eg.Wait()
	if err != nil {
		runerr = err
		return
	}

	if !cfg.KeepService {
		ow.Info("cleaning up finished pods...")
	}

	runerr = nil
	return
}

func (*ClusterK8sRunner) ID() string {
	return "cluster:k8s"
}

func (c *ClusterK8sRunner) Healthcheck(ctx context.Context, engine api.Engine, ow *rpc.OutputWriter, fix bool) (*api.HealthcheckReport, error) {
	// Ignore sync client error as we may start the redis pod below.
	if err := c.initPool(); err != nil && !errors.Is(err, errSyncClient) {
		return nil, err
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// How many plan worker nodes are there?
	res, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "testground.node.role.plan=true",
	})
	if err != nil {
		return nil, err
	}
	planNodes := res.Items

	hh := &healthcheck.Helper{}

	hh.Enlist("efs pod",
		healthcheck.CheckK8sPods(ctx, client, "app=efs-provisioner", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("redis pod",
		healthcheck.CheckK8sPods(ctx, client, "app=redis", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("sync service pod",
		healthcheck.CheckK8sPods(ctx, client, "name=testground-sync-service", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("prometheus pod",
		healthcheck.CheckK8sPods(ctx, client, "app=prometheus", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("grafana pod",
		healthcheck.CheckK8sPods(ctx, client, "app.kubernetes.io/name=grafana", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("sidecar pods",
		healthcheck.CheckK8sPods(ctx, client, "name=testground-sidecar", c.config.Namespace, len(planNodes)),
		healthcheck.NotImplemented(),
	)

	return hh.RunChecks(ctx, fix)

}

func (*ClusterK8sRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(ClusterK8sRunnerConfig{})
}

func (*ClusterK8sRunner) CompatibleBuilders() []string {
	return []string{"docker:go", "docker:generic"}
}

func (c *ClusterK8sRunner) Enabled() bool {
	_ = c.initPool()
	return c.pool != nil
}

func (c *ClusterK8sRunner) initPool() error {
	mu.Lock()
	defer mu.Unlock()

	if c.initialized {
		return nil
	}

	c.config = defaultKubernetesConfig()
	c.imagesLRU, _ = lru.New(256)

	var err error
	workers := 20

	c.pool, err = newPool(workers, c.config)
	if err != nil {
		return err
	}

	c.syncClient, err = ss.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return fmt.Errorf("%w: %s", errSyncClient, err)
	}

	c.initialized = true
	return nil
}

func (c *ClusterK8sRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	if err := c.initPool(); err != nil {
		return fmt.Errorf("could not init pool: %w", err)
	}

	log := ow.With("runner", "cluster:k8s", "run_id", input.RunID)
	err := c.ensureCollectOutputsPod(ctx, input)
	if err != nil {
		return err
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// This is the same line found in client_pool.go...
	// I need the restCfg, for remotecommand.
	// TODO: Reorganize not to repeat ourselves.
	k8sCfg, err := clientcmd.BuildConfigFromFlags("", c.config.KubeConfigPath)
	if err != nil {
		return err
	}

	// This request is sent to the collect-outputs pod
	// tar, compress, and write to stdout.
	// stdout will remain connected so we can read it later.

	log.Info("collecting outputs")

	req := client.
		CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Name("collect-outputs").
		Namespace("default").
		SubResource("exec").
		Param("container", "collect-outputs").
		VersionedParams(&v1.PodExecOptions{
			Container: "collect-outputs",
			Command: []string{
				"tar",
				"-C",
				"/outputs",
				"-czf",
				"-",
				input.RunID,
			},
			Stdin:  false,
			Stderr: false,
			Stdout: true,
		}, scheme.ParameterCodec)

	log.Debug("sending command to remote server: ", req.URL())
	exec, err := remotecommand.NewSPDYExecutor(k8sCfg, "POST", req.URL())
	if err != nil {
		log.Warnf("failed to send remote collection command: %v", err)
		return err
	}

	// Connect stdout of the above command to the output file
	// Connect stderr to a buffer which we can read from to display any errors to the user.
	outbuf := bufio.NewWriter(ow.BinaryWriter())
	defer outbuf.Flush()
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: outbuf,
	})
	if err != nil {
		log.Warnf("failed to collect results from remote collection command: %v", err)
		return err
	}
	return nil
}

// waitForPod waits until a given pod reaches the desired `phase` or the context is canceled
func (c *ClusterK8sRunner) waitForPod(ctx context.Context, podName string, phase string) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	var p string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if p == phase {
				return nil
			}
			res, err := client.CoreV1().Pods(c.config.Namespace).List(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + podName,
			})
			if err != nil {
				return err
			}
			if len(res.Items) != 1 {
				continue
			}

			pod := res.Items[0]
			p = string(pod.Status.Phase)

			time.Sleep(1 * time.Second)
		}
	}
}

// ensureCollectOutputsPod ensures that we have a collect-outputs pod running
func (c *ClusterK8sRunner) ensureCollectOutputsPod(ctx context.Context, input *api.CollectionInput) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	res, err := client.CoreV1().Pods(c.config.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + collectOutputsPodName,
	})
	if err != nil {
		return err
	}
	if len(res.Items) == 0 {
		err = c.createCollectOutputsPod(ctx, input)
		if err != nil {
			return err
		}
		err = c.waitForPod(ctx, collectOutputsPodName, "Running")
		if err != nil {
			return err
		}
	} else if len(res.Items) > 1 {
		return errors.New("unexpected number of pods for outputs collection")
	}

	return nil
}

func (c *ClusterK8sRunner) getPodLogs(ow *rpc.OutputWriter, podName string) (string, error) {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	podLogOpts := v1.PodLogOptions{
		LimitBytes: int64Ptr(10000000000), // 100mb
	}

	var podLogs io.ReadCloser
	var err error
	err = retry(5, 5*time.Second, func() error {
		req := client.CoreV1().Pods(c.config.Namespace).GetLogs(podName, &podLogOpts)
		podLogs, err = req.Stream(context.TODO())
		if err != nil {
			ow.Warnw("got error when trying to fetch pod logs", "err", err.Error())
		}
		return err
	})
	if err != nil {
		return "", fmt.Errorf("error in opening stream: %v", err)
	}
	defer podLogs.Close()

	lr := byline.NewReader(podLogs)
	lr.MapString(func(line string) string { return podName + " | " + line })

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, lr)
	if err != nil {
		return "", fmt.Errorf("error in copy information from podLogs to buf: %v", err)
	}

	return buf.String(), nil
}

func (c *ClusterK8sRunner) watchRunPods(ctx context.Context, ow *rpc.OutputWriter, input *api.RunInput, result *Result, rp *runtime.RunParams) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	cfg := *input.RunnerConfig.(*ClusterK8sRunnerConfig)

	runTimeout := 10 * time.Minute
	if cfg.RunTimeoutMin != 0 {
		runTimeout = time.Duration(cfg.RunTimeoutMin) * time.Minute
	}

	fieldSelector := "type!=Normal"
	opts := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}
	eventsWatcher, err := client.CoreV1().Events(c.config.Namespace).Watch(ctx, opts)
	if err != nil {
		ow.Errorw("k8s client pods list error", "err", err.Error())
		return err
	}
	defer eventsWatcher.Stop()
	eventsChan := eventsWatcher.ResultChan()

	go func() {
		for ge := range eventsChan {
			e, ok := ge.Object.(*v1.Event)

			if ok && strings.Contains(e.InvolvedObject.Name, input.RunID) {
				id := e.ObjectMeta.Name

				event := fmt.Sprintf("obj<%s> type<%s> reason<%s> message<%s> type<%s> count<%d> lastTimestamp<%s>", e.InvolvedObject.Name, ge.Type, e.Reason, e.Message, e.Type, e.Count, e.LastTimestamp)

				ow.Warnw("testplan received event", "event", event)

				result.Journal.Events[id] = event
			}
		}
	}()

	podsByState := make(map[string]*v1.PodList)
	var countersMu sync.Mutex

	start := time.Now()
	allRunningStage := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Since(start) > runTimeout {
			return fmt.Errorf("run timeout reached. make sure your plan execution completes within %s.", runTimeout)
		}
		time.Sleep(2000 * time.Millisecond)

		countPodsByState := func(state string) int {
			fieldSelector := fmt.Sprintf("status.phase=%s", state)
			opts := metav1.ListOptions{
				LabelSelector: fmt.Sprintf("testground.run_id=%s", input.RunID),
				FieldSelector: fieldSelector,
			}
			res, err := client.CoreV1().Pods(c.config.Namespace).List(ctx, opts)
			if err != nil {
				ow.Warnw("k8s client pods list error", "err", err.Error())
				return -1
			}
			countersMu.Lock()
			podsByState[state] = res
			countersMu.Unlock()
			return len(res.Items)
		}

		counters := map[string]int{}
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

		ow.Debugw("testplan pods state", "running_for", time.Since(start).Truncate(time.Second), "succeeded", counters["Succeeded"], "running", counters["Running"], "pending", counters["Pending"], "failed", counters["Failed"], "unknown", counters["Unknown"])

		if counters["Failed"] > 0 {
			for _, p := range podsByState["Failed"].Items {
				if !strings.Contains(p.ObjectMeta.Name, input.RunID) {
					continue
				}

				for _, st := range p.Status.ContainerStatuses {
					event := fmt.Sprintf("pod status <failed> obj<%s> reason<%s> started_at<%s> finished_at<%s> exitcode<%d>", st.Name, st.State.Terminated.Reason, st.State.Terminated.StartedAt, st.State.Terminated.FinishedAt, st.State.Terminated.ExitCode)
					ow.Warnw("testplan received status", "status", event)
					result.Journal.PodsStatuses[event] = struct{}{}
				}
			}
		}

		if counters["Running"] == input.TotalInstances && !allRunningStage {
			allRunningStage = true
			ow.Infow("all testplan instances in `Running` state", "took", time.Since(start).Truncate(time.Second))
		}

		if counters["Succeeded"] == input.TotalInstances {
			ow.Infow("all testplan instances in `Succeeded` state", "took", time.Since(start).Truncate(time.Second))
			return nil
		}

		if (counters["Succeeded"] + counters["Failed"]) == input.TotalInstances {
			ow.Warnw("all testplan instances in `Succeeded` or `Failed` state", "took", time.Since(start).Truncate(time.Second))
			return nil
		}
	}
}

func (c *ClusterK8sRunner) createTestplanPod(ctx context.Context, podName string, input *api.RunInput, runenv runtime.RunParams, env []v1.EnvVar, g *api.RunGroup, i int, podResourceMemory resource.Quantity, podResourceCPU resource.Quantity) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	cfg := *input.RunnerConfig.(*ClusterK8sRunnerConfig)

	var ports []v1.ContainerPort
	cnt := 0
	for _, p := range cfg.ExposedPorts {
		port, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return err
		}

		ports = append(ports, v1.ContainerPort{Name: fmt.Sprintf("port%d", cnt), ContainerPort: int32(port)})
		cnt++
	}

	mountPropagationMode := v1.MountPropagationHostToContainer
	sharedVolumeName := "efs-shared"

	podRequest := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"testground.plan":     input.TestPlan,
				"testground.testcase": runenv.TestCase,
				"testground.run_id":   input.RunID,
				"testground.groupid":  g.ID,
				"testground.purpose":  "plan",
			},
			Annotations: map[string]string{"cni": defaultK8sNetworkAnnotation, "k8s.v1.cni.cncf.io/networks": "weave"},
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: sharedVolumeName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "efs",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			InitContainers: []v1.Container{
				{
					Name:            "wait-for-sidecar",
					Image:           "busybox",
					ImagePullPolicy: v1.PullIfNotPresent,
					Args:            []string{"-c", "until nc -vz $HOST_IP 6060; do echo \"Waiting for local sidecar to listen to $HOST_IP:6060\"; sleep 2; done;"},
					Command:         []string{"sh"},
					Env:             env,
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Mi"),
							v1.ResourceCPU:    resource.MustParse("10m"),
						},
					},
				},
				{
					Name:            "mkdir-outputs",
					Image:           "busybox",
					ImagePullPolicy: v1.PullIfNotPresent,
					Args:            []string{"-c", "mkdir -p $TEST_OUTPUTS_PATH"},
					Command:         []string{"sh"},
					Env:             env,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             sharedVolumeName,
							MountPath:        "/outputs",
							MountPropagation: &mountPropagationMode,
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("10Mi"),
							v1.ResourceCPU:    resource.MustParse("10m"),
						},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:            podName,
					Image:           g.ArtifactPath,
					ImagePullPolicy: v1.PullIfNotPresent,
					Args:            []string{},
					Env:             env,
					Ports:           ports,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             sharedVolumeName,
							MountPath:        "/outputs",
							MountPropagation: &mountPropagationMode,
						},
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: podResourceMemory,
							v1.ResourceCPU:    podResourceCPU,
						},
						Limits: v1.ResourceList{
							v1.ResourceMemory: podResourceMemory,
						},
					},
				},
			},
			NodeSelector: map[string]string{"testground.node.role.plan": "true"},
		},
	}

	_, err := client.CoreV1().Pods(c.config.Namespace).Create(ctx, podRequest, metav1.CreateOptions{})
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

// checkClusterResources returns whether we can fit the input groups in the current cluster
func (c *ClusterK8sRunner) checkClusterResources(ow *rpc.OutputWriter, groups []*api.RunGroup, fallbackMemory resource.Quantity, fallbackCPU resource.Quantity) (bool, error) {
	neededCPUs := 0.0

	defaultPodCPU, err := strconv.ParseFloat(fallbackCPU.AsDec().String(), 64)
	if err != nil {
		return false, err
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	res, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "testground.node.role.plan=true",
	})
	if err != nil {
		return false, err
	}

	nodes := len(res.Items)

	// all worker nodes are the same, so just take allocatable CPU from the first
	item := res.Items[0].Status.Allocatable["cpu"]
	nodeCPUs := item.ToDec().Value()

	totalCPUs := nodes * int(nodeCPUs)

	availableCPUs := float64(totalCPUs) - float64(nodes)*sidecarCPUs

	for _, g := range groups {
		var podCPU float64
		if g.Resources.CPU != "" {
			cpu, err := resource.ParseQuantity(g.Resources.CPU)
			if err != nil {
				return false, err
			}
			podCPU, err = strconv.ParseFloat(cpu.AsDec().String(), 64)
			if err != nil {
				return false, err
			}
		} else {
			podCPU = defaultPodCPU
		}

		neededCPUs += podCPU * float64(g.Instances)
	}

	if (availableCPUs * utilisation) > neededCPUs {
		return true, nil
	}

	ow.Warnw("not enough resources on cluster", "available_cpus", availableCPUs, "needed_cpus", neededCPUs, "utilisation", utilisation)
	return false, nil
}

// TerminateAll terminates all pods for with the label testground.purpose: plan
// This command will remove all plan pods in the cluster.
func (c *ClusterK8sRunner) TerminateAll(ctx context.Context, ow *rpc.OutputWriter) error {
	if err := c.initPool(); err != nil {
		return fmt.Errorf("could not init pool: %w", err)
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	planPods := metav1.ListOptions{
		LabelSelector: "testground.purpose=plan",
	}
	err := client.CoreV1().Pods("default").DeleteCollection(ctx, metav1.DeleteOptions{}, planPods)
	if err != nil {
		ow.Errorw("could not terminate all pods", "err", err)
		return err
	}
	return nil
}

func (c *ClusterK8sRunner) pushImagesToDockerRegistry(ctx context.Context, ow *rpc.OutputWriter, in *api.RunInput) error {
	cfg := *in.RunnerConfig.(*ClusterK8sRunnerConfig)

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	start := time.Now()
	ow.Info("pushing images")
	defer func() { ow.Infow("pushing of images finished", "took", time.Since(start).Truncate(time.Second)) }()

	var ipo types.ImagePushOptions // Auth params for Docker client
	var uri string                 // URI of Docker registry to push images to

	switch cfg.Provider {
	case "aws":
		// Setup docker registry authentication
		auth, err := aws.ECR.GetAuthToken(in.EnvConfig.AWS)
		if err != nil {
			return err
		}
		ow.Infow("acquired ECR authentication token")

		ipo = types.ImagePushOptions{
			RegistryAuth: aws.ECR.EncodeAuthToken(auth),
		}

		// Setup docker registry repository
		repo := fmt.Sprintf("testground-%s-%s", in.EnvConfig.AWS.Region, in.TestPlan)
		uri, err = aws.ECR.EnsureRepository(in.EnvConfig.AWS, repo)
		if err != nil {
			return err
		}
		ow.Infow("ensured ECR repository exists", "name", repo)

	case "dockerhub":
		// Setup docker registry authentication
		auth := types.AuthConfig{
			Username: in.EnvConfig.DockerHub.Username,
			Password: in.EnvConfig.DockerHub.AccessToken,
		}
		authBytes, err := json.Marshal(auth)
		if err != nil {
			return err
		}
		authBase64 := base64.URLEncoding.EncodeToString(authBytes)

		ipo = types.ImagePushOptions{
			RegistryAuth: authBase64,
		}

		// Setup docker registry repository
		uri = in.EnvConfig.DockerHub.Repo + "/testground"

	default:
		return fmt.Errorf("unknown provider: %s", cfg.Provider)
	}

	return c.pushToDockerRegistry(ctx, ow, cli, in, ipo, uri)
}

func (c *ClusterK8sRunner) createCollectOutputsPod(ctx context.Context, input *api.CollectionInput) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	cfg := *input.RunnerConfig.(*ClusterK8sRunnerConfig)

	collectOutputsCPU, err := resource.ParseQuantity(cfg.CollectOutputsPodCPU)
	if err != nil {
		return fmt.Errorf("couldn't parse default `collect` pod CPU request; make sure you have specified `collect_outputs_pod_cpu` in .env.toml; err: %w", err)
	}

	collectOutputsMemory, err := resource.ParseQuantity(cfg.CollectOutputsPodMemory)
	if err != nil {
		return fmt.Errorf("couldn't parse default `collect` pod Memory request; make sure you have specified `collect_outputs_pod_memory` in .env.toml; err: %w", err)
	}

	mountPropagationMode := v1.MountPropagationHostToContainer
	sharedVolumeName := "efs-shared"

	podRequest := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: collectOutputsPodName,
			Labels: map[string]string{
				"testground.purpose": "outputs",
			},
			Annotations: map[string]string{"cni": defaultK8sNetworkAnnotation, "k8s.v1.cni.cncf.io/networks": "ipvlan-multus"},
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: sharedVolumeName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: "efs",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"testground.node.role.infra": "true",
			},
			Containers: []v1.Container{
				{
					Name:    "collect-outputs",
					Image:   "busybox",
					Args:    []string{"-c", "sleep 999999999"},
					Command: []string{"sh"},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             sharedVolumeName,
							MountPath:        "/outputs",
							MountPropagation: &mountPropagationMode,
						},
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    collectOutputsCPU,
							v1.ResourceMemory: collectOutputsMemory,
						},
						Limits: v1.ResourceList{
							v1.ResourceMemory: collectOutputsMemory,
						},
					},
				},
			},
		},
	}

	_, err = client.CoreV1().Pods(c.config.Namespace).Create(ctx, podRequest, metav1.CreateOptions{})
	return err
}

func (c *ClusterK8sRunner) GetClusterCapacity() (int64, int64, error) {
	if err := c.initPool(); err != nil {
		return -1, -1, fmt.Errorf("could not init pool: %w", err)
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	res, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "testground.node.role.plan=true",
	})
	if err != nil {
		return 0, 0, err
	}

	var allocatableCPUs int64
	var allocatableMemory int64
	var capacityCPUs int64
	var capacityMemory int64

	for _, it := range res.Items {
		i := it.Status.Allocatable["cpu"]
		r, _ := i.AsInt64()
		allocatableCPUs += r

		i = it.Status.Allocatable["memory"]
		r, _ = i.AsInt64()
		allocatableMemory += r

		i = it.Status.Capacity["cpu"]
		r, _ = i.AsInt64()
		capacityCPUs += r

		i = it.Status.Capacity["memory"]
		r, _ = i.AsInt64()
		capacityMemory += r
	}

	return allocatableCPUs, allocatableMemory, nil
}

func (c *ClusterK8sRunner) collectOutcomes(ctx context.Context, result *Result, tpl *runtime.RunParams) (chan bool, error) {
	eventsCh, err := c.syncClient.SubscribeEvents(ctx, tpl)
	if err != nil {
		return nil, err
	}

	done := make(chan bool)

	go func() {
		running := true
		for running {
			select {
			case <-ctx.Done():
				running = false
			case e := <-eventsCh:
				// for now we emit only outcome OK events, so no need for more checks
				if e.SuccessEvent != nil {
					se := e.SuccessEvent
					o := result.Outcomes[se.TestGroupID]
					o.Ok = o.Ok + 1
				}
			}
		}

		result.Outcome = task.OutcomeSuccess
		if len(result.Outcomes) == 0 {
			result.Outcome = task.OutcomeFailure
		}

		for g := range result.Outcomes {
			if result.Outcomes[g].Total != result.Outcomes[g].Ok {
				result.Outcome = task.OutcomeFailure
				break
			}
		}

		done <- true
	}()

	return done, nil
}

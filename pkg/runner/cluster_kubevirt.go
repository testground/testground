package runner

import (
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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/msoap/byline"
	"github.com/testground/sdk-go/ptypes"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/testground/sdk-go/runtime"
	ss "github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/aws"
	"github.com/testground/testground/pkg/conv"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"
	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	lru "github.com/hashicorp/golang-lru"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"text/template"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	v1 "kubevirt.io/client-go/kubevirt/typed/core/v1"
)

var (
	_               api.Runner        = (*ClusterKubevirtRunner)(nil)
	_               api.Terminatable  = (*ClusterKubevirtRunner)(nil)
	_               api.Healthchecker = (*ClusterKubevirtRunner)(nil)
	kubevirtMu                        = sync.Mutex{}
	errKVSyncClient                   = errors.New("failed to start sync client")
)

const (
	defaultKubevirtNetworkAnnotation = "aws-cni"
	// collect-outputs pod is used to compress outputs at the end of a testplan run
	// as well as to copy archives from it, since it has EFS attached to it
	collectOutputsVMName = "collect-outputs"

	// number of CPUs allocated to each Sidecar. should be same as what is set in sidecar.yaml
	kvSidecarCPUs = 0.2

	// utilisation is how many CPUs from the remainder shall we allocate to Testground
	// note that there are other services running on the Kubernetes cluster such as
	// api proxy, node_exporter, dummy, etc.
	kvUtilisation = 0.85

	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check sdk/sync for more information
	kvNetworkInitialisationSuccessful = "network initialisation successful"
	kvNetworkInitialisationFailed     = "network initialisation failed"
)

var kubevirtSubnetIdx uint64 = 0

func init() {
	// Avoid collisions in picking up subnets
	rand.Seed(time.Now().UnixNano())
	kubevirtSubnetIdx = rand.Uint64() % 4096
}

func nextKubevirtSubnet() (*net.IPNet, error) {
	subnet, _, err := nextDataNetwork(int(atomic.AddUint64(&kubevirtSubnetIdx, 1) % 4096))
	if err != nil {
		return nil, err
	}
	return subnet, err
}

func getHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// ClusterKubevirtRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type ClusterKubevirtRunnerConfig struct {
	// LogLevel sets the log level in the test VMs (default: not set).
	LogLevel string `toml:"log_level"`

	KeepService bool `toml:"keep_service"`

	// Provider is the infrastructure provider to use
	Provider string `toml:"provider"`

	// Whether Kubernetes cluster has an autoscaler running
	AutoscalerEnabled bool `toml:"autoscaler_enabled"`

	// Resources requested for each testplan VM from the Kubernetes cluster
	TestplanVmMemory string `toml:"testplan_vm_memory"`
	TestplanVmCPU    string `toml:"testplan_vm_cpu"`

	// Resources requested for the `collect-outputs` VM from the Kubernetes cluster
	CollectOutputsVmMemory string `toml:"collect_outputs_vm_memory"`
	CollectOutputsVmCPU    string `toml:"collect_outputs_vm_cpu"`

	ExposedPorts ExposedPorts `toml:"exposed_ports"`

	RunTimeoutMin int `toml:"run_timeout_min"`

	Sysctls []string `toml:"sysctls"`

	// Base VM image to use for test VMs (optional, defaults to using the test image)
	// Only used for utility VMs like collect-outputs
	BaseVmImage string `toml:"base_vm_image,omitempty"`

	// Scripts to run at different stages of VM lifecycle
	Scripts struct {
		// Script to run during cloud-init (first boot)
		Init string `toml:"init"`
		// Script to run before starting the test
		PreTest string `toml:"pre_test"`
		// Script to run after the test completes
		PostTest string `toml:"post_test"`
	} `toml:"scripts"`

	// Environment variables to set in the VM
	EnvVars map[string]string `toml:"env_vars"`

	Namespace string `toml:"namespace"`

	// LocalRegistry is the address of the local registry to push images to when provider = "local".
	LocalRegistry string `toml:"local_registry,omitempty"` // e.g., "localhost:5000"
}

// ClusterKubevirtRunner is a runner that creates KubeVirt virtual machines to run test instances.
type ClusterKubevirtRunner struct {
	initialized bool
	config      KubernetesConfig
	pool        *kvPool
	kvClient    kubecli.KubevirtClient
	imagesLRU   *lru.Cache
	syncClient  *ss.DefaultClient
}

// kvPool is a pool of Kubernetes and KubeVirt client connections
type kvPool struct {
	availableC chan kubecli.KubevirtClient
}

// newKVPool returns a pool of KubeVirt client connections
func newKVPool(workers int, config KubernetesConfig) (*kvPool, error) {
	k8sCfg, err := clientcmd.BuildConfigFromFlags("", config.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not start k8s client from config: %v", err)
	}

	pool := &kvPool{
		availableC: make(chan kubecli.KubevirtClient, workers),
	}

	for i := 0; i < workers; i++ {
		kvClient, err := kubecli.GetKubevirtClientFromRESTConfig(k8sCfg)
		if err != nil {
			return nil, fmt.Errorf("could not create kubevirt client: %v", err)
		}

		pool.availableC <- kvClient
	}

	return pool, nil
}

func (p *kvPool) Acquire() kubecli.KubevirtClient {
	return <-p.availableC
}

func (p *kvPool) Release(cs kubecli.KubevirtClient) {
	p.availableC <- cs
}

func (c *ClusterKubevirtRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (runoutput *api.RunOutput, runerr error) {
	if err := c.initPool(); err != nil {
		return nil, fmt.Errorf("could not init pool: %w", err)
	}

	cfg := *input.RunnerConfig.(*ClusterKubevirtRunnerConfig)
	if cfg.Namespace != "" {
		c.config.Namespace = cfg.Namespace
	}
	logging.S().Infow("KubeVirt runner using namespace", "namespace", c.config.Namespace)

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

	ow = ow.With("runner", "cluster:kubevirt", "run_id", input.RunID)

	// if `provider` is set, we have to push to a docker registry
	if cfg.Provider != "" {
		logging.S().Infow("pushing images to docker registry provider", "uri", cfg.Provider)
		err := c.pushImagesToDockerRegistry(ctx, ow, input)
		if err != nil {
			runerr = fmt.Errorf("failed to push images to %s; err: %w", cfg.Provider, err)
			return
		}
	}

	defaultCPU, err := resource.ParseQuantity(cfg.TestplanVmCPU)
	if err != nil {
		runerr = fmt.Errorf("couldn't parse default test plan VM CPU request; make sure you have specified `testplan_vm_cpu` in .env.toml; err: %w", err)
		return
	}

	defaultMemory, err := resource.ParseQuantity(cfg.TestplanVmMemory)
	if err != nil {
		runerr = fmt.Errorf("couldn't parse default test plan VM Memory request; make sure you have specified `testplan_vm_memory` in .env.toml; err: %w", err)
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

	// Get subnet for test instances
	subnet, err := nextKubevirtSubnet()
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

	vmName := fmt.Sprintf("tg-%s", input.TestPlan)

	ow.Infow("deploying testground testplan run on kubevirt", "vm-name", vmName)

	var eg errgroup.Group

	eg.Go(func() error {
		ctxVMs, cancel := context.WithCancel(ctx)
		defer cancel()

		outcomesDoneCh, err := c.collectOutcomes(ctxVMs, result, &template)
		if err != nil {
			ow.Errorw("could not start collecting outcomes", "err", err)
		}

		err = c.watchRunVMs(ctx, ow, input, result, &template)
		if err != nil {
			return err
		}

		cancel()
		<-outcomesDoneCh
		return nil
	})

	sem := make(chan struct{}, 30) // limit the number of concurrent k8s/kubevirt api calls

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
		env = append(env, corev1.EnvVar{Name: "REDIS_HOST", Value: "testground-infra-redis"})
		env = append(env, corev1.EnvVar{Name: "SYNC_SERVICE_HOST", Value: "testground-sync-service"})
		env = append(env, corev1.EnvVar{Name: "INFLUXDB_URL", Value: "http://influxdb:8086"})
		// This subnet should correspond to the secondary CNI's IP range (usually Weave)
		env = append(env, corev1.EnvVar{Name: "TEST_SUBNET", Value: "10.32.0.0/12"})

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, corev1.EnvVar{Name: "LOG_LEVEL", Value: cfg.LogLevel})
		}

		env = append(env, corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}})
		env = append(env, corev1.EnvVar{Name: "HOST_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"}}})

		// Inject exposed ports.
		for name, value := range cfg.ExposedPorts.ToEnvVars() {
			env = append(env, corev1.EnvVar{Name: name, Value: value})
		}

		vmCPU := defaultCPU
		if g.Resources.CPU != "" {
			var err error
			vmCPU, err = resource.ParseQuantity(g.Resources.CPU)
			if err != nil {
				runerr = err
				return
			}
		}

		vmMemory := defaultMemory
		if g.Resources.Memory != "" {
			var err error
			vmMemory, err = resource.ParseQuantity(g.Resources.Memory)
			if err != nil {
				runerr = err
				return
			}
		}

		for i := 0; i < g.Instances; i++ {
			// Context cancellation check
			select {
			case <-ctx.Done():
				runerr = ctx.Err()
				return
			default:
			}

			i := i
			g := g
			sem <- struct{}{}

			vmName := fmt.Sprintf("%s-%s-%s-%d", vmName, input.RunID, g.ID, i)

			defer func() {
				if cfg.KeepService {
					return
				}
				client := c.pool.Acquire()
				defer c.pool.Release(client)
				ow.Debugw("deleting vm", "vm", vmName)
				// K8s equivalent: client.CoreV1().Pods(c.config.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
				err = client.VirtualMachine(c.config.Namespace).Delete(ctx, vmName, metav1.DeleteOptions{})
				if err != nil {
					ow.Errorw("couldn't remove vm", "vm", vmName, "err", err)
				}
			}()

			eg.Go(func() error {
				defer func() { <-sem }()

				// Context cancellation check inside goroutine
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				currentEnv := make([]corev1.EnvVar, len(env))
				copy(currentEnv, env)

				currentEnv = append(currentEnv, corev1.EnvVar{
					Name:  "TEST_OUTPUTS_PATH",
					Value: fmt.Sprintf("/outputs/%s/%s/%d", input.RunID, g.ID, i),
				})

				return c.createTestplanVM(ctx, vmName, input, runenv, currentEnv, g, i, vmMemory, vmCPU)
			})
		}
	}

	// we want to fetch logs even in an event of error
	defer func() {
		if input.TotalInstances <= 200 {
			var gg errgroup.Group

			for _, g := range input.Groups {
				for i := 0; i < g.Instances; i++ {
					// Context cancellation check
					select {
					case <-ctx.Done():
						return
					default:
					}

					i := i
					g := g
					sem <- struct{}{}

					gg.Go(func() error {
						defer func() { <-sem }()

						// Context cancellation check inside goroutine
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
						}

						vmName := fmt.Sprintf("%s-%s-%s-%d", vmName, input.RunID, g.ID, i)

						err := c.waitForVMIRunning(ctx, vmName, 5*time.Minute)
						if err != nil {
							return err
						}
						ow.Debugw("fetching logs", "vm", vmName)
						logs, err := c.getVMLogs(ow, vmName, input.RunID)
						if err != nil {
							return err
						}
						ow.Debugw("got logs", "vm", vmName, "len", len(logs))

						_, err = ow.WriteProgress([]byte(logs))
						return err
					})
				}
			}

			ow.Debugw("waiting for logs")

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
		ow.Info("cleaning up finished vms...")
	}

	runerr = nil
	return
}

func (*ClusterKubevirtRunner) ID() string {
	return "cluster:kubevirt"
}

func (*ClusterKubevirtRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(ClusterKubevirtRunnerConfig{})
}

func (*ClusterKubevirtRunner) CompatibleBuilders() []string {
	return []string{"docker:kubevirt"}
}

func (c *ClusterKubevirtRunner) Enabled() bool {
	_ = c.initPool()
	return c.pool != nil
}

func (c *ClusterKubevirtRunner) initPool() error {
	kubevirtMu.Lock()
	defer kubevirtMu.Unlock()

	if c.initialized {
		return nil
	}

	c.config = defaultKubernetesConfig()
	c.imagesLRU, _ = lru.New(256)

	var err error
	workers := 20

	c.pool, err = newKVPool(workers, c.config)
	if err != nil {
		return err
	}

	c.syncClient, err = ss.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return fmt.Errorf("%w: %s", errKVSyncClient, err)
	}

	c.initialized = true
	return nil
}

// createTestplanVM creates a KubeVirt VM to run a test instance
// K8s equivalent: client.CoreV1().Pods(c.config.Namespace).Create(ctx, podRequest, metav1.CreateOptions{})
func (c *ClusterKubevirtRunner) createTestplanVM(ctx context.Context, vmName string, input *api.RunInput, runenv runtime.RunParams, env []corev1.EnvVar, g *api.RunGroup, i int, vmResourceMemory resource.Quantity, vmResourceCPU resource.Quantity) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	logging.S().Infow("creating testplan VM",
		"vm_name", vmName,
		"run_id", input.RunID,
		"group_id", g.ID,
		"instance", i,
		"memory", vmResourceMemory.String(),
		"cpu", vmResourceCPU.String())

	cfg := *input.RunnerConfig.(*ClusterKubevirtRunnerConfig)

	vmImage := g.ArtifactPath
	if vmImage == "" {
		logging.S().Errorw("test image not found", "vm_name", vmName)
		return fmt.Errorf("test image must be pushed to registry before creating VM")
	}

	logging.S().Debugw("using test image as VM disk",
		"vm_name", vmName,
		"image", vmImage)

	var sysctls []corev1.Sysctl
	for _, v := range cfg.Sysctls {
		sysctl := strings.Split(v, "=")
		sysctls = append(sysctls, corev1.Sysctl{Name: sysctl[0], Value: sysctl[1]})
	}

	var ports []corev1.ContainerPort
	cnt := 0
	for _, p := range cfg.ExposedPorts {
		port, err := strconv.ParseInt(p, 10, 32)
		if err != nil {
			return err
		}
		ports = append(ports, corev1.ContainerPort{Name: fmt.Sprintf("port%d", cnt), ContainerPort: int32(port)})
		cnt++
	}

	sharedVolumeName := "outputs"
	rootDiskName := "rootdisk"
	cloudInitName := "cloudinit"

	// --- NEW: Render cloud-init from template ---
	type CloudInitTemplateData struct {
		Username      string
		Password      string
		EnvVars       map[string]string
		RunID         string
		InstanceType  string
		InstanceIndex int
	}

	envVars := map[string]string{}
	for k, v := range cfg.EnvVars {
		safeKey := strings.ReplaceAll(k, "-", "_")
		envVars[safeKey] = v
	}
	for _, e := range env {
		if e.Value != "" {
			safeKey := strings.ReplaceAll(e.Name, "-", "_")
			envVars[safeKey] = e.Value
		}
	}

	cloudInitData := CloudInitTemplateData{
		Username:      "testground",
		Password:      "123456",
		EnvVars:       envVars,
		RunID:         input.RunID,
		InstanceType:  g.ID,
		InstanceIndex: i,
	}

	// open file for cloud init tmpl
	tmplBytes, err := os.ReadFile("/templates/cloud-init.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read cloud-init template: %w", err)
	}
	tmpl, err := template.New("cloud-init").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse cloud-init template: %w", err)
	}
	var userDataBuf bytes.Buffer
	if err := tmpl.Execute(&userDataBuf, cloudInitData); err != nil {
		return fmt.Errorf("failed to render cloud-init template: %w", err)
	}
	userData := userDataBuf.String()

	logging.S().Debugw("cloud-init user-data", "user_data", userData)

	var cloudInit *kubevirtv1.CloudInitNoCloudSource
	const cloudInitUserDataLimit = 2048
	if len(userData) > cloudInitUserDataLimit {
		// UserData too large, create a Secret and use UserDataSecretRef
		secretName := fmt.Sprintf("cloudinit-%s", vmName)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: c.config.Namespace,
			},
			Data: map[string][]byte{
				"userdata": []byte(userData),
			},
			Type: corev1.SecretTypeOpaque,
		}
		clientset := client.CoreV1()

		logging.S().Debugw("creating cloud-init secret", "secret_name", secretName)
		_, err := clientset.Secrets(c.config.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create cloud-init secret: %w", err)
		}
		cloudInit = &kubevirtv1.CloudInitNoCloudSource{
			UserDataSecretRef: &corev1.LocalObjectReference{
				Name: secretName,
			},
		}
	} else {
		// UserData fits, use inline
		cloudInit = &kubevirtv1.CloudInitNoCloudSource{
			UserData: userData,
		}
	}
	// --- END NEW ---

	// Create a KubeVirt VirtualMachine object
	vmRequest := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: vmName,
			Labels: map[string]string{
				"testground.plan":     input.TestPlan,
				"testground.testcase": runenv.TestCase,
				"testground.run_id":   input.RunID,
				"testground.groupid":  g.ID,
				"testground.purpose":  "plan",
			},
			Annotations: map[string]string{"cni": defaultKubevirtNetworkAnnotation, "k8s.v1.cni.cncf.io/networks": "weave"},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Running: ptr(true),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"testground.plan":     input.TestPlan,
						"testground.testcase": runenv.TestCase,
						"testground.run_id":   input.RunID,
						"testground.groupid":  g.ID,
						"testground.purpose":  "plan",
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Resources: kubevirtv1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: vmResourceMemory,
								corev1.ResourceCPU:    vmResourceCPU,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: vmResourceMemory,
							},
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: rootDiskName,
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
								{
									Name: sharedVolumeName,
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
									Serial: "shared-storage",
								},
								{
									Name: cloudInitName,
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: rootDiskName,
							VolumeSource: kubevirtv1.VolumeSource{
								ContainerDisk: &kubevirtv1.ContainerDiskSource{
									Image:           vmImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
							},
						},
						{
							Name: sharedVolumeName,
							VolumeSource: kubevirtv1.VolumeSource{
								EmptyDisk: &kubevirtv1.EmptyDiskSource{
									Capacity: resource.MustParse("10Gi"),
								},
							},
						},
						{
							Name: cloudInitName,
							VolumeSource: kubevirtv1.VolumeSource{
								CloudInitNoCloud: cloudInit,
							},
						},
					},
					NodeSelector: map[string]string{"testground.node.role.plan": "true"},
				},
			},
		},
	}

	logging.S().Debugw("creating VM with configuration",
		"vm_name", vmName,
		"labels", vmRequest.ObjectMeta.Labels,
		"annotations", vmRequest.ObjectMeta.Annotations)

	_, err = client.VirtualMachine(c.config.Namespace).Create(ctx, vmRequest, metav1.CreateOptions{})
	if err != nil {
		logging.S().Errorw("failed to create VM",
			"vm_name", vmName,
			"error", err)
		return err
	}

	logging.S().Infow("successfully created VM",
		"vm_name", vmName,
		"namespace", c.config.Namespace)

	return nil
}

func ptr[T any](v T) *T {
	return &v
}

func (c *ClusterKubevirtRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	if err := c.initPool(); err != nil {
		return fmt.Errorf("could not init pool: %w", err)
	}

	log := ow.With("runner", "cluster:kubevirt", "run_id", input.RunID)
	err := c.ensureCollectOutputsVM(ctx, ow, input)
	if err != nil {
		return err
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	log.Info("collecting outputs")

	// Get VM logs via SerialConsole
	stream, err := client.VirtualMachineInstance(c.config.Namespace).SerialConsole(collectOutputsVMName, &v1.SerialConsoleOptions{})
	if err != nil {
		log.Warnw("failed to get VM logs via serial console", "err", err.Error())
		return err
	}

	err = stream.Stream(v1.StreamOptions{
		In:  bytes.NewBufferString(fmt.Sprintf("tar -C /outputs -czf - %s\n", input.RunID)),
		Out: ow.BinaryWriter(),
	})
	if err != nil {
		log.Warnw("failed to collect results", "err", err.Error())
		return err
	}

	return nil
}

func (c *ClusterKubevirtRunner) TerminateAll(ctx context.Context, ow *rpc.OutputWriter) error {
	if err := c.initPool(); err != nil {
		return fmt.Errorf("could not init pool: %w", err)
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// List all VMs with the plan label
	vmiList, err := client.VirtualMachineInstance(c.config.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "testground.purpose=plan",
	})
	if err != nil {
		ow.Errorw("could not list VMs", "err", err)
		return err
	}

	// Delete each VM
	for _, vmi := range vmiList.Items {
		err := client.VirtualMachineInstance(c.config.Namespace).Delete(ctx, vmi.Name, metav1.DeleteOptions{})
		if err != nil {
			ow.Errorw("could not delete VM", "name", vmi.Name, "err", err)
		}
	}

	return nil
}

func (c *ClusterKubevirtRunner) Healthcheck(ctx context.Context, engine api.Engine, ow *rpc.OutputWriter, fix bool) (*api.HealthcheckReport, error) {
	// Ignore sync client error as we may start the redis service below.
	if err := c.initPool(); err != nil && !errors.Is(err, errKVSyncClient) {
		return nil, err
	}

	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// Check for KubeVirt installation
	_, err := client.ExtensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(ctx, "virtualmachineinstances.kubevirt.io", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("KubeVirt CRDs not found, is KubeVirt installed? Error: %w", err)
	}

	// How many worker nodes are there?
	res, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "testground.node.role.plan=true",
	})
	if err != nil {
		return nil, err
	}
	planNodes := res.Items

	hh := &healthcheck.Helper{}

	hh.Enlist("efs service",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "app=efs-provisioner", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("redis service",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "app=redis", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("sync service",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "name=testground-sync-service", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("prometheus service",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "app=prometheus", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("grafana service",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "app.kubernetes.io/name=grafana", c.config.Namespace, 1),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("sidecar services",
		healthcheck.CheckK8sPods(ctx, client.CoreV1().Pods(c.config.Namespace), "name=testground-sidecar", c.config.Namespace, len(planNodes)),
		healthcheck.NotImplemented(),
	)

	hh.Enlist("kubevirt api",
		func() (bool, string, error) {
			info, err := client.ServerVersion().Get()
			if err != nil {
				return false, "", err
			}
			return true, fmt.Sprintf("KubeVirt version: %s", info.GitVersion), nil
		},
		healthcheck.NotImplemented(),
	)

	return hh.RunChecks(ctx, fix)
}

func (c *ClusterKubevirtRunner) pushImagesToDockerRegistry(ctx context.Context, ow *rpc.OutputWriter, in *api.RunInput) error {
	cfg := *in.RunnerConfig.(*ClusterKubevirtRunnerConfig)

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

	case "local":
		// Support for local registry (e.g., localhost:5000 or user-specified)
		// Add a config entry: local_registry = "localhost:5000" (default)
		uri = cfg.LocalRegistry
		if uri == "" {
			uri = "localhost:5000"
		}
		// Set RegistryAuth to base64-encoded empty JSON object for unauthenticated registry
		ipo = types.ImagePushOptions{
			RegistryAuth: base64.StdEncoding.EncodeToString([]byte("{}")),
		}
		ow.Infow("using local registry", "uri", uri)

	default:
		return fmt.Errorf("unknown provider: %s", cfg.Provider)
	}

	return c.pushToDockerRegistry(ctx, ow, cli, in, ipo, uri)
}

func (c *ClusterKubevirtRunner) pushToDockerRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.RunInput, ipo types.ImagePushOptions, uri string) error {
	for _, g := range in.Groups {
		repo := in.TestPlan // use the test plan name as the repository
		tag := fmt.Sprintf("%s/%s:%s", uri, repo, g.ArtifactPath)

		if _, ok := c.imagesLRU.Get(tag); ok {
			ow.Infow("image already pushed and tagged", "group_id", g.ID, "tag", tag)
			g.ArtifactPath = tag
			continue
		}

		ow.Infow("tagging image", "group_id", g.ID, "tag", tag)
		if err := client.ImageTag(ctx, g.ArtifactPath, tag); err != nil {
			return err
		}

		ow.Infow("pushing image for group", "group_id", g.ID, "tag", tag)
		rc, err := client.ImagePush(ctx, tag, ipo)
		if err != nil {
			return err
		}

		if _, err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
			return err
		}

		c.imagesLRU.Add(tag, struct{}{})

		// replace the artifact path by the pushed image.
		g.ArtifactPath = tag
		ow.Infow("pushed image for group", "group_id", g.ID, "tag", tag)
	}

	return nil
}

func (c *ClusterKubevirtRunner) checkClusterResources(ow *rpc.OutputWriter, groups []*api.RunGroup, fallbackMemory resource.Quantity, fallbackCPU resource.Quantity) (bool, error) {
	neededCPUs := 0.0

	defaultVmCPU, err := strconv.ParseFloat(fallbackCPU.AsDec().String(), 64)
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

	availableCPUs := float64(totalCPUs) - float64(nodes)*kvSidecarCPUs

	for _, g := range groups {
		var vmCPU float64
		if g.Resources.CPU != "" {
			cpu, err := resource.ParseQuantity(g.Resources.CPU)
			if err != nil {
				return false, err
			}
			vmCPU, err = strconv.ParseFloat(cpu.AsDec().String(), 64)
			if err != nil {
				return false, err
			}
		} else {
			vmCPU = defaultVmCPU
		}

		neededCPUs += vmCPU * float64(g.Instances)
	}

	if (availableCPUs * kvUtilisation) > neededCPUs {
		return true, nil
	}

	ow.Warnw("not enough resources on cluster", "available_cpus", availableCPUs, "needed_cpus", neededCPUs, "utilisation", kvUtilisation)
	return false, nil
}

func (c *ClusterKubevirtRunner) collectOutcomes(ctx context.Context, result *Result, tpl *runtime.RunParams) (chan bool, error) {
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

func (c *ClusterKubevirtRunner) watchRunVMs(ctx context.Context, ow *rpc.OutputWriter, input *api.RunInput, result *Result, rp *runtime.RunParams) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	logging.S().Infow("starting to watch VMs",
		"run_id", input.RunID,
		"total_instances", input.TotalInstances)

	cfg := *input.RunnerConfig.(*ClusterKubevirtRunnerConfig)

	runTimeout := 10 * time.Minute
	if cfg.RunTimeoutMin != 0 {
		runTimeout = time.Duration(cfg.RunTimeoutMin) * time.Minute
	}

	logging.S().Debugw("configured run timeout",
		"timeout", runTimeout.String())

	// Watch for VM events
	fieldSelector := fmt.Sprintf("involvedObject.kind=VirtualMachineInstance")
	opts := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}
	eventsWatcher, err := client.CoreV1().Events(c.config.Namespace).Watch(ctx, opts)
	if err != nil {
		logging.S().Errorw("failed to watch VM events",
			"error", err,
			"namespace", c.config.Namespace,
			"event_field_selector", fieldSelector,
			"rbac_hint", "Ensure the service account has get/list/watch on events in this namespace")
		ow.Errorw("k8s client events watch error", "err", fmt.Sprintf("%+v", err))
		return err
	}
	defer eventsWatcher.Stop()
	eventsChan := eventsWatcher.ResultChan()

	go func() {
		for ge := range eventsChan {
			e, ok := ge.Object.(*corev1.Event)

			if ok && strings.Contains(e.InvolvedObject.Name, input.RunID) {
				id := e.ObjectMeta.Name

				event := fmt.Sprintf("obj<%s> type<%s> reason<%s> message<%s> type<%s> count<%d> lastTimestamp<%s>",
					e.InvolvedObject.Name, ge.Type, e.Reason, e.Message, e.Type, e.Count, e.LastTimestamp)

				logging.S().Debugw("received VM event",
					"vm_name", e.InvolvedObject.Name,
					"event_type", ge.Type,
					"reason", e.Reason,
					"message", e.Message)

				ow.Warnw("testplan received event", "event", event)

				result.Journal.Events[id] = event
			}
		}
	}()

	vmsByState := make(map[kubevirtv1.VirtualMachineInstancePhase]*kubevirtv1.VirtualMachineInstanceList)
	var countersMu sync.Mutex

	start := time.Now()
	allRunningStage := false
	for {
		select {
		case <-ctx.Done():
			logging.S().Infow("context canceled, stopping VM watch",
				"run_id", input.RunID)
			return ctx.Err()
		default:
		}

		if time.Since(start) > runTimeout {
			logging.S().Errorw("run timeout reached",
				"run_id", input.RunID,
				"timeout", runTimeout.String())
			return fmt.Errorf("run timeout reached. make sure your plan execution completes within %s.", runTimeout)
		}
		time.Sleep(2000 * time.Millisecond)

		countVMsByState := func(state kubevirtv1.VirtualMachineInstancePhase) int {
			opts := metav1.ListOptions{
				LabelSelector: fmt.Sprintf("testground.run_id=%s", input.RunID),
			}
			res, err := client.VirtualMachineInstance(c.config.Namespace).List(ctx, opts)
			if err != nil {
				logging.S().Errorw("failed to list VMs",
					"error", err,
					"state", state,
					"namespace", c.config.Namespace)
				ow.Warnw("kubevirt client VMs list error", "err", err.Error())
				return -1
			}

			var count int
			var matchingVMs kubevirtv1.VirtualMachineInstanceList
			matchingVMs.TypeMeta = res.TypeMeta
			matchingVMs.ListMeta = res.ListMeta

			for _, vm := range res.Items {
				if vm.Status.Phase == state {
					matchingVMs.Items = append(matchingVMs.Items, vm)
					count++
				}
			}

			countersMu.Lock()
			vmsByState[state] = &matchingVMs
			countersMu.Unlock()

			return count
		}

		counters := map[kubevirtv1.VirtualMachineInstancePhase]int{}
		states := []kubevirtv1.VirtualMachineInstancePhase{
			kubevirtv1.Pending,
			kubevirtv1.Scheduling,
			kubevirtv1.Scheduled,
			kubevirtv1.Running,
			kubevirtv1.Succeeded,
			kubevirtv1.Failed,
		}

		var wg sync.WaitGroup
		wg.Add(len(states))
		for _, state := range states {
			state := state
			go func() {
				defer wg.Done()

				n := countVMsByState(state)

				countersMu.Lock()
				counters[state] = n
				countersMu.Unlock()
			}()
		}
		wg.Wait()

		ow.Debugw("testplan VMs state",
			"running_for", time.Since(start).Truncate(time.Second),
			"succeeded", counters[kubevirtv1.Succeeded],
			"running", counters[kubevirtv1.Running],
			"pending", counters[kubevirtv1.Pending],
			"scheduling", counters[kubevirtv1.Scheduling],
			"scheduled", counters[kubevirtv1.Scheduled],
			"failed", counters[kubevirtv1.Failed])

		if counters[kubevirtv1.Failed] > 0 {
			countersMu.Lock()
			for _, vm := range vmsByState[kubevirtv1.Failed].Items {
				if !strings.Contains(vm.ObjectMeta.Name, input.RunID) {
					continue
				}

				event := fmt.Sprintf("VM status <failed> obj<%s> phase<%s>", vm.Name, kubevirtv1.Failed)
				ow.Warnw("testplan received status", "status", event)
				result.Journal.PodsStatuses[event] = struct{}{}
			}
			countersMu.Unlock()
		}

		if counters[kubevirtv1.Running] == input.TotalInstances && !allRunningStage {
			allRunningStage = true
			ow.Infow("all testplan instances in `Running` state", "took", time.Since(start).Truncate(time.Second))
		}

		if counters[kubevirtv1.Succeeded] == input.TotalInstances {
			ow.Infow("all testplan instances in `Succeeded` state", "took", time.Since(start).Truncate(time.Second))
			return nil
		}

		if (counters[kubevirtv1.Succeeded] + counters[kubevirtv1.Failed]) == input.TotalInstances {
			ow.Warnw("all testplan instances in `Succeeded` or `Failed` state", "took", time.Since(start).Truncate(time.Second))
			return nil
		}
	}
}

func (c *ClusterKubevirtRunner) getVMLogs(ow *rpc.OutputWriter, vmName string, runID string) (string, error) {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// Get VM logs via SerialConsole
	stream, err := client.VirtualMachineInstance(c.config.Namespace).SerialConsole(vmName, &v1.SerialConsoleOptions{})
	if err != nil {
		ow.Warnw("failed to get VM logs via serial console", "err", err.Error())
		return "", err
	}
	defer stream.AsConn().Close()

	err = stream.Stream(v1.StreamOptions{
		In:  bytes.NewBufferString(fmt.Sprintf("tar -C /outputs -czf - %s\n", runID)),
		Out: ow.BinaryWriter(),
	})
	if err != nil {
		ow.Warnw("failed to collect results", "err", err.Error())
		return "", err
	}

	buf := new(bytes.Buffer)
	lr := byline.NewReader(stream.AsConn())

	_, err = io.Copy(buf, lr)
	if err != nil {
		ow.Warnw("failed to collect results", "err", err.Error())
		return "", err
	}

	return buf.String(), nil
}

func (c *ClusterKubevirtRunner) ensureCollectOutputsVM(ctx context.Context, ow *rpc.OutputWriter, input *api.CollectionInput) error {
	cfg := *input.RunnerConfig.(*ClusterKubevirtRunnerConfig)
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	// Parse resource requests with fallbacks
	vmResourceMemory := resource.MustParse("512Mi") // fallback
	if cfg.CollectOutputsVmMemory != "" {
		vmResourceMemory = resource.MustParse(cfg.CollectOutputsVmMemory)
	}

	vmResourceCPU := resource.MustParse("500m") // fallback
	if cfg.CollectOutputsVmCPU != "" {
		vmResourceCPU = resource.MustParse(cfg.CollectOutputsVmCPU)
	}

	vmName := collectOutputsVMName
	vmis := client.VirtualMachineInstance(c.config.Namespace)

	// Check if VM already exists and is healthy
	existingVMI, err := vmis.Get(ctx, vmName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to check if collect-outputs VM exists: %w", err)
	} else if err == nil {
		// VM exists, check if it's healthy
		if existingVMI.Status.Phase == kubevirtv1.Running {
			return nil
		}
		// VM exists but not healthy, delete it
		err = vmis.Delete(ctx, vmName, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete unhealthy collect-outputs VM: %w", err)
		}
		// Wait for deletion to complete
		for {
			_, err := vmis.Get(ctx, vmName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to check VM deletion status: %w", err)
			}
			time.Sleep(2 * time.Second)
		}
	}

	// First ensure the PVC exists
	pvcName := "outputs-pvc"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				"testground.purpose": "outputs",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("50Gi"),
				},
			},
			StorageClassName: ptr("efs-sc"),
		},
	}

	_, err = client.CoreV1().PersistentVolumeClaims(c.config.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create outputs PVC: %w", err)
	}

	// Wait for PVC to be bound
	timeout := 2 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for PVC to be bound")
		}
		pvc, err := client.CoreV1().PersistentVolumeClaims(c.config.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to check PVC status: %w", err)
		}
		if pvc.Status.Phase == corev1.ClaimBound {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Create cloud-init config with better error handling and logging
	userData := `#cloud-config
write_files:
- path: /var/log/testground-init.log
  permissions: '0644'
runcmd:
  - |
    set -ex
    exec 1>/var/log/testground-init.log 2>&1
    echo "Starting testground outputs initialization"
    mkdir -p /outputs/%s/%s/%d
    echo "Mounting outputs volume"
    mount /dev/vdb /outputs || {
      echo "Failed to mount outputs volume"
      exit 1
    }
    echo "Setting permissions"
    chmod -R 777 /outputs/%s
    echo "Initialization complete"
    touch /outputs/.ready`

	// Create the VM with proper configuration
	vmi := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				"testground.purpose": "outputs",
				"testground.type":    "collect-outputs",
			},
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Resources: kubevirtv1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: vmResourceMemory,
						corev1.ResourceCPU:    vmResourceCPU,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: vmResourceMemory,
						corev1.ResourceCPU:    vmResourceCPU,
					},
				},
				Devices: kubevirtv1.Devices{
					Disks: []kubevirtv1.Disk{
						{
							Name: "rootdisk",
							DiskDevice: kubevirtv1.DiskDevice{
								Disk: &kubevirtv1.DiskTarget{
									Bus: "virtio",
								},
							},
						},
						{
							Name: "outputs",
							DiskDevice: kubevirtv1.DiskDevice{
								Disk: &kubevirtv1.DiskTarget{
									Bus: "virtio",
								},
							},
						},
						{
							Name: "cloudinit",
							DiskDevice: kubevirtv1.DiskDevice{
								Disk: &kubevirtv1.DiskTarget{
									Bus: "virtio",
								},
							},
						},
					},
				},
			},
			Volumes: []kubevirtv1.Volume{
				{
					Name: "rootdisk",
					VolumeSource: kubevirtv1.VolumeSource{
						ContainerDisk: &kubevirtv1.ContainerDiskSource{
							Image:           cfg.BaseVmImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
				{
					Name: "outputs",
					VolumeSource: kubevirtv1.VolumeSource{
						PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
							PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcName,
							},
						},
					},
				},
				{
					Name: "cloudinit",
					VolumeSource: kubevirtv1.VolumeSource{
						CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
							UserData: userData,
						},
					},
				},
			},
			NodeSelector: map[string]string{
				"testground.node.role.infra": "true",
			},
			TerminationGracePeriodSeconds: ptr(int64(30)),
		},
	}

	// Create the VM
	_, err = vmis.Create(ctx, vmi, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create collect-outputs VM: %w", err)
	}

	// Wait for VM to be ready with a timeout
	timeout = 5 * time.Minute
	if err := c.waitForVMIRunning(ctx, vmName, timeout); err != nil {
		return fmt.Errorf("collect-outputs VM failed to become ready within %s: %w", timeout, err)
	}

	// Wait for outputs directory to be ready
	deadline = time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for outputs directory to be ready")
		}

		// Check if .ready file exists in outputs directory
		cmd := []string{"test", "-f", "/outputs/.ready"}
		_, err := c.execInVM(ctx, vmName, cmd)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return nil
}

// execInVM executes a command in the VM and returns its output
func (c *ClusterKubevirtRunner) execInVM(ctx context.Context, vmName string, command []string) (string, error) {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	stdout := new(bytes.Buffer)

	// Use the proper KubeVirt API for executing commands
	console, err := client.VirtualMachineInstance(c.config.Namespace).SerialConsole(vmName, &v1.SerialConsoleOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get serial console: %w", err)
	}

	// Execute command through serial console
	cmdStr := strings.Join(command, " ")
	err = console.Stream(v1.StreamOptions{
		In:  bytes.NewBufferString(cmdStr + "\n"),
		Out: stdout,
	})
	if err != nil {
		return "", fmt.Errorf("failed to execute command in VM: %w", err)
	}

	return stdout.String(), nil
}

func (c *ClusterKubevirtRunner) waitForVMIRunning(ctx context.Context, vmName string, timeout time.Duration) error {
	client := c.pool.Acquire()
	defer c.pool.Release(client)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for VMI %s to be running: %w", vmName, ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout after %v waiting for VMI %s to be running", timeout, vmName)
			}

			vmi, err := client.VirtualMachineInstance(c.config.Namespace).Get(ctx, vmName, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue // VM not created yet, keep waiting
				}
				return fmt.Errorf("failed to get VMI %s status: %w", vmName, err)
			}

			switch vmi.Status.Phase {
			case kubevirtv1.Running:
				return nil
			case kubevirtv1.Failed:
				return fmt.Errorf("VMI %s entered Failed state: %s", vmName, vmi.Status.Reason)
			case kubevirtv1.Unknown:
				return fmt.Errorf("VMI %s entered Unknown state: %s", vmName, vmi.Status.Reason)
			default:
				// Other states (Pending, Scheduling, etc) are normal progression
				continue
			}
		}
	}
}

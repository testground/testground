package runner

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/testground/testground/pkg/logging"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/testground/sdk-go/ptypes"

	"github.com/testground/sdk-go/runtime"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/conv"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/imdario/mergo"
	"golang.org/x/sync/errgroup"

	ss "github.com/testground/sdk-go/sync"
)

const InfraMaxFilesUlimit int64 = 1048576

var (
	_ api.Runner        = (*LocalDockerRunner)(nil)
	_ api.Healthchecker = (*LocalDockerRunner)(nil)
	_ api.Terminatable  = (*LocalDockerRunner)(nil)
)

// LocalDockerRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type LocalDockerRunnerConfig struct {
	// KeepContainers retains test containers even after they exit (default:
	// false).
	KeepContainers bool `toml:"keep_containers"`
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`
	// Unstarted creates the containers without starting them (default: false).
	Unstarted bool `toml:"no_start"`
	// Background avoids tailing the output of containers, and displaying it as
	// log messages (default: false).
	Background bool `toml:"background"`
	// Ulimits that should be applied on this run, in Docker format.
	// See
	// https://docs.docker.com/engine/reference/commandline/run/#set-ulimits-in-container---ulimit
	// (default: ["nofile=1048576:1048576"]).
	Ulimits []string `toml:"ulimits"`

	// AdditionalEnvs can be used to set additional environment variables.
	// Note to use unique environment variables not used by Testground,
	// as your custom environment variables will get overwritten by the ones
	// also set by Testground.
	AdditionalEnvs map[string]string `toml:"additional_envs"`

	ExposedPorts ExposedPorts `toml:"exposed_ports"`
	// Collection timeout is the time we wait for the sync service to send us the test outcomes after
	// all instances have finished.
	OutcomesCollectionTimeout time.Duration `toml:"outcomes_collection_timeout"`

	AdditionalHosts []string `toml:"additional_hosts"`
}

type testContainerInstance struct {
	containerID string
	groupID     string
	groupIdx    int
}

// defaultConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultConfig = LocalDockerRunnerConfig{
	KeepContainers:            false,
	Unstarted:                 false,
	Background:                false,
	Ulimits:                   []string{"nofile=1048576:1048576"},
	OutcomesCollectionTimeout: time.Second * 45,
}

// LocalDockerRunner is a runner that manually stands up as many docker
// containers as instances the run job indicates.
//
// It creates a user-defined bridge, to which it attaches a redis service, and
// all the containers that belong to this test case. It then monitors all test
// containers, and destroys the setup once all workloads are done.
//
// What we do here is slightly similar to what Docker Compose does, but we can't
// use the latter because it's a python program and it doesn't expose a network
// API.
type LocalDockerRunner struct {
	lk sync.RWMutex

	controlNetworkID string
	outputsDir       string

	syncClient *ss.DefaultClient
}

func (r *LocalDockerRunner) Healthcheck(ctx context.Context, engine api.Engine, ow *rpc.OutputWriter, fix bool) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	r.outputsDir = filepath.Join(engine.EnvConfig().Dirs().Outputs(), "local_docker")
	r.controlNetworkID = "testground-control"

	hh := &healthcheck.Helper{}

	// enlist healthchecks which are common between local:docker and local:exec
	localCommonHealthcheck(ctx, hh, cli, ow, r.controlNetworkID, r.outputsDir)

	dockerSock := "/var/run/docker.sock"
	if host := cli.DaemonHost(); strings.HasPrefix(host, "unix://") {
		dockerSock = host[len("unix://"):]
	} else {
		ow.Warnf("guessing docker socket as %s", dockerSock)
	}

	additionalHosts := "ADDITIONAL_HOSTS="
	envHosts, hasHosts := engine.EnvConfig().Runners["local:docker"]["additional_hosts"].([]string)
	if hasHosts {
		additionalHosts += strings.Join(envHosts, ",")
	}
	sidecarContainerOpts := docker.EnsureContainerOpts{
		ContainerName: "testground-sidecar",
		ContainerConfig: &container.Config{
			Image:      "iptestground/sidecar:edge",
			Entrypoint: []string{"testground"},
			Cmd:        []string{"sidecar", "--runner", "docker"},
			// NOTE: we export REDIS_HOST for compatibility with older sdk versions.
			Env: []string{"SYNC_SERVICE_HOST=testground-sync-service", "REDIS_HOST=testground-redis", "INFLUXDB_HOST=testground-influxdb", "GODEBUG=gctrace=1", additionalHosts},
		},
		HostConfig: &container.HostConfig{
			PublishAllPorts: true,
			// Port binding for pprof.
			PortBindings: nat.PortMap{"6060": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}}},
			NetworkMode:  container.NetworkMode(r.controlNetworkID),
			// To lookup namespaces. Can't use SandboxKey for some reason.
			PidMode: "host",
			// We need _both_ to actually get a network namespace handle.
			// We may be able to drop sys_admin if we drop netlink
			// sockets that we're not using.
			CapAdd: []string{"NET_ADMIN", "SYS_ADMIN"},
			// needed to talk to docker.
			Mounts: []mount.Mount{{
				Type:   mount.TypeBind,
				Source: dockerSock,
				Target: "/var/run/docker.sock",
			}},
			Resources: container.Resources{
				Ulimits: []*units.Ulimit{
					{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
				},
			},
			RestartPolicy: container.RestartPolicy{
				Name: "unless-stopped",
			},
		},
	}

	// sidecar healthcheck.
	hh.Enlist("sidecar-container",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-sidecar"),
		healthcheck.StartContainer(ctx, ow, cli, &sidecarContainerOpts),
	)

	// RunChecks will fill the report and return any errors.
	return hh.RunChecks(ctx, fix)
}

// setupSyncClient sets up the sync client if it is not set up already.
func (r *LocalDockerRunner) setupSyncClient() error {
	r.lk.Lock()
	defer r.lk.Unlock()

	if r.syncClient != nil {
		return nil
	}

	err := os.Setenv(ss.EnvServiceHost, "127.0.0.1")
	if err != nil {
		return err
	}

	r.syncClient, err = ss.NewGenericClient(context.Background(), logging.S())
	if err != nil {
		return err
	}

	return nil
}

// collectOutcomes listens to the sync service and collects the outcome for every test instance.
// It stops when all instances have submitted a result or the context was canceled.
func (r *LocalDockerRunner) collectOutcomes(ctx context.Context, result *Result, tpl *runtime.RunParams) (chan bool, error) {
	eventsCh, err := r.syncClient.SubscribeEvents(ctx, tpl)
	if err != nil {
		return nil, err
	}

	// TODO: eventually we'll keep a trace of each test instance status.
	// Right now, if a container sends multiple events, it will mess up the outcomes.
	// We have to pass its group id to the container, so that it can send us back messages
	// with its own id.
	expectingOutcomes := result.countTotalInstances()
	done := make(chan bool)

	go func() {
		running := true
		for running && expectingOutcomes > 0 {
			select {
			case <-ctx.Done():
				running = false
			case e := <-eventsCh:
				if e.SuccessEvent != nil {
					result.addOutcome(e.SuccessEvent.TestGroupID, task.OutcomeSuccess)
					expectingOutcomes -= 1
				} else if e.FailureEvent != nil {
					result.addOutcome(e.FailureEvent.TestGroupID, task.OutcomeFailure)
					expectingOutcomes -= 1
				} else if e.CrashEvent != nil {
					result.addOutcome(e.CrashEvent.TestGroupID, task.OutcomeFailure)
					expectingOutcomes -= 1
				}
				// else: skip
			}
		}

		result.updateOutcome()
		done <- true
	}()

	return done, nil
}

func (r *LocalDockerRunner) prepareOutputDirectory(instance_id int, runenv *runtime.RunParams) (string, error) {
	// <outputs_dir>/<plan>/<run_id>/<group_id>/<instance_number>
	odir := filepath.Join(r.outputsDir, runenv.TestPlan, runenv.TestRun, runenv.TestGroupID, strconv.Itoa(instance_id))

	err := os.MkdirAll(odir, 0777)
	if err != nil {
		return "", fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
	}

	return odir, nil
}

func (r *LocalDockerRunner) prepareTemporaryDirectory(instance_id int, runenv *runtime.RunParams) (string, error) {
	var tmpdir string
	tmpdir, err := ioutil.TempDir("", "testground")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s: %w", tmpdir, err)
	}

	return tmpdir, nil
}

func (r *LocalDockerRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (runoutput *api.RunOutput, err error) {
	log := ow.With("runner", "local:docker", "run_id", input.RunID)

	result := newResult(input)
	runoutput = &api.RunOutput{
		RunID:  input.RunID,
		Result: result,
	}

	defer func() {
		if err != nil && result.Outcome == "" {
			result.Outcome = task.OutcomeFailure
		}
		if ctx.Err() == context.Canceled {
			result.Outcome = task.OutcomeCanceled
		}
	}()

	err = r.setupSyncClient()
	if err != nil {
		log.Error(err)
		return
	}

	// Grab a read lock. This will allow many runs to run simultaneously, but
	// they will be exclusive of state-altering healthchecks.
	// TODO: I'm not sure this is true anymore.
	r.lk.RLock()
	defer r.lk.RUnlock()

	// ## Prepare Execution Context

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}

	// Create a data network.
	dataNetworkID, subnet, err := newDataNetwork(ctx, cli, ow, input, "default")
	if err != nil {
		return
	}

	// Prepare the Run Environment template.
	template := runtime.RunParams{
		TestPlan:           input.TestPlan,
		TestCase:           input.TestCase,
		TestRun:            input.RunID,
		TestInstanceCount:  input.TotalInstances,
		TestDisableMetrics: input.DisableMetrics,
		TestSidecar:        true,
		TestOutputsPath:    "/outputs",
		TestTempPath:       "/temp", // not using /tmp to avoid overriding linux standard paths.
		TestStartTime:      time.Now(),
		TestSubnet:         &ptypes.IPNet{IPNet: *subnet},
	}

	// Prepare the Runner Configuration.
	cfg := defaultConfig
	if err = mergo.Merge(&cfg, input.RunnerConfig, mergo.WithOverride); err != nil {
		err = fmt.Errorf("error while merging configurations: %w", err)
		return
	}

	// Prepare the ports mapping.
	ports := make(nat.PortSet)
	for _, p := range cfg.ExposedPorts {
		ports[nat.Port(p)] = struct{}{}
	}

	// Prepare environment variables.
	sharedEnv := make([]string, 0, 3)
	sharedEnv = append(sharedEnv, "INFLUXDB_URL=http://testground-influxdb:8086")
	sharedEnv = append(sharedEnv, "REDIS_HOST=testground-redis")
	// Inject exposed ports.
	sharedEnv = append(sharedEnv, conv.ToOptionsSlice(cfg.ExposedPorts.ToEnvVars())...)
	// Set the log level if provided in cfg.
	if cfg.LogLevel != "" {
		sharedEnv = append(sharedEnv, "LOG_LEVEL="+cfg.LogLevel)
	}

	// ## Create the containers
	var (
		containers []testContainerInstance
		tmpdirs    []string
	)

	defer func() {
		// remove all temporary directories.
		for _, tmpdir := range tmpdirs {
			_ = os.RemoveAll(tmpdir)
		}
	}()

	for _, g := range input.Groups {
		reviewResources(g, ow)

		runenv := template
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestGroupID = g.ID
		runenv.TestInstanceParams = g.Parameters
		runenv.TestCaptureProfiles = g.Profiles
		// Prepare the group's environment variables.
		env := make([]string, 0, len(cfg.AdditionalEnvs)+len(sharedEnv)+len(runenv.ToEnvVars()))
		env = append(env, conv.ToOptionsSlice(cfg.AdditionalEnvs)...)
		env = append(env, sharedEnv...)
		env = append(env, conv.ToOptionsSlice(runenv.ToEnvVars())...)
		logging.S().Infow("additional hosts", "hosts", strings.Join(cfg.AdditionalHosts, ","))
		env = append(env, fmt.Sprintf("ADDITIONAL_HOSTS=%s", strings.Join(cfg.AdditionalHosts, ",")))

		// Start as many containers as group instances.
		for i := 0; i < g.Instances; i++ {
			// TODO: We should set the instance id in runenv and make this whole operation self contained around a local runenv.
			tmpdir, err := r.prepareTemporaryDirectory(i, &runenv)
			if err != nil {
				return nil, fmt.Errorf("failed to prepare temporary directory: %w", err)
			}
			tmpdirs = append(tmpdirs, tmpdir)

			odir, err := r.prepareOutputDirectory(i, &runenv)
			if err != nil {
				return nil, fmt.Errorf("failed to prepare output directory: %w", err)
			}

			// TODO: runenv.TestRun == input.RunID. Refactor into a single name.
			name := fmt.Sprintf("tg-%s-%s-%s-%s-%d", runenv.TestPlan, runenv.TestCase, runenv.TestRun, runenv.TestGroupID, i)
			log.Infow("creating container", "name", name)

			ccfg := &container.Config{
				Image:        g.ArtifactPath,
				ExposedPorts: ports,
				Env:          env,
				Labels: map[string]string{
					"testground.purpose":  "plan",
					"testground.plan":     runenv.TestPlan,
					"testground.testcase": runenv.TestCase,
					"testground.run_id":   runenv.TestRun,
					"testground.group_id": runenv.TestGroupID,
				},
			}

			hcfg := &container.HostConfig{
				NetworkMode:     container.NetworkMode("testground-control"),
				PublishAllPorts: true,
				Mounts: []mount.Mount{{
					Type:   mount.TypeBind,
					Source: odir,
					Target: runenv.TestOutputsPath,
				}, {
					Type:   mount.TypeBind,
					Source: tmpdir,
					Target: runenv.TestTempPath,
				}},
			}

			if len(cfg.Ulimits) > 0 {
				ulimits, err := conv.ToUlimits(cfg.Ulimits)
				if err == nil {
					hcfg.Resources = container.Resources{Ulimits: ulimits}
				} else {
					ow.Warnf("invalid ulimit will be ignored %v", err)
				}
			}

			// Create the container.
			var res container.ContainerCreateCreatedBody
			res, err = cli.ContainerCreate(ctx, ccfg, hcfg, nil, name)
			if err != nil {
				break
			}

			container := testContainerInstance{
				containerID: res.ID,
				groupID:     g.ID,
				groupIdx:    i,
			}
			containers = append(containers, container)

			// TODO: Remove this when we get the sidecar working. It'll do this for us.
			err = attachContainerToNetwork(ctx, cli, res.ID, dataNetworkID)
			if err != nil {
				break
			}
		}
	}

	if !cfg.KeepContainers {
		defer func() {
			ids := make([]string, 0, len(containers))
			for _, c := range containers {
				ids = append(ids, c.containerID)
			}
			if err := docker.DeleteContainers(cli, log, ids); err != nil {
				log.Errorw("failed to delete containers", "err", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := cli.NetworkRemove(ctx, dataNetworkID); err != nil {
				log.Errorw("removing network", "network", dataNetworkID, "error", err)
			}
		}()
	}

	// If an error occurred interim, abort.
	if err != nil {
		log.Error(err)
		return
	}

	if cfg.Unstarted {
		return
	}

	// ## Start the containers & log their outputs.
	runCtx, cancelRun := context.WithCancel(ctx)

	defer func() {
		cancelRun()
	}()

	// First we collect every container outcomes.
	outcomesCollectIsCompleteCh, err := r.collectOutcomes(runCtx, result, &template)
	if err != nil {
		log.Error(err)
		return
	}

	// Second we start the containers
	log.Infow("starting containers", "count", len(containers))
	var (
		startGroup, startGroupCtx = errgroup.WithContext(runCtx)
		runGroup, runGroupCtx     = errgroup.WithContext(runCtx)
		ratelimit                 = make(chan struct{}, 16)
		started                   = make(chan testContainerInstance, len(containers))
	)

	for _, c := range containers {
		c := c
		f := func() error {
			ratelimit <- struct{}{}
			defer func() { <-ratelimit }()

			log.Infow("starting container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)

			err := cli.ContainerStart(startGroupCtx, c.containerID, types.ContainerStartOptions{})
			if err == nil {
				log.Debugw("started container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)
				select {
				case <-startGroupCtx.Done():
				default:
					started <- c
				}
			}

			return err
		}
		startGroup.Go(f)
	}

	// Third we start the pretty printer
	if !cfg.Background {
		pretty := NewPrettyPrinter(ow)

		// Tail the sidecar container logs and appends them to the pretty printer.
		go func() {
			t := time.Now().Add(time.Duration(-10) * time.Second) // sidecar is a long running daemon, so we care only about logs around the execution of our test run
			stream, err := cli.ContainerLogs(runCtx, "testground-sidecar", types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: false,
				Since:      t.Format("2006-01-02T15:04:05"),
				Follow:     true,
			})

			if err != nil {
				log.Errorw("failed to attach sidecar", "error", err)
				cancelRun()
				return
			}

			rstdout, wstdout := io.Pipe()
			rstderr, wstderr := io.Pipe()
			go func() {
				_, _ = stdcopy.StdCopy(wstdout, wstderr, stream)
				_ = wstdout.Close()
				_ = wstderr.Close()
			}()

			pretty.Append("sidecar     ", rstdout, rstderr)
		}()

		// Tail the other container logs and appends them to the pretty printer.
		// This goroutine takes started containers and attaches them to the pretty printer.
		go func() {
			for {
				select {
				case c := <-started:
					log.Infow("attaching container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)
					stream, err := cli.ContainerLogs(runCtx, c.containerID, types.ContainerLogsOptions{
						ShowStdout: true,
						ShowStderr: true,
						Since:      "2019-01-01T00:00:00",
						Follow:     true,
					})

					if err != nil {
						log.Errorw("failed to attach container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx, "error", err)
						cancelRun()
						return
					}

					rstdout, wstdout := io.Pipe()
					rstderr, wstderr := io.Pipe()
					go func() {
						_, err := stdcopy.StdCopy(wstdout, wstderr, stream)
						_ = wstdout.CloseWithError(err)
						_ = wstderr.CloseWithError(err)
					}()

					// instance tag in output: << group[zero_padded_i] >> (container_id[0:6]), e.g. << miner[003] (a1b2c3) >>
					tag := fmt.Sprintf("%s[%03d] (%s)", c.groupID, c.groupIdx, c.containerID[0:6])
					pretty.Manage(tag, rstdout, rstderr)
				case <-runCtx.Done():
					// Exit
					return
				}
			}
		}()
	}

	// Wait for all container to have started
	err = startGroup.Wait()
	if err != nil {
		log.Error(err)
		return
	}

	// Finally, we're going to follow our containers until they are done

	for _, c := range containers {
		c := c
		f := func() error {
			log.Infow("waiting for container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)

			statusCh, errCh := cli.ContainerWait(runCtx, c.containerID, container.WaitConditionNotRunning)

			select {
			case err := <-errCh:
				log.Infow("container failed", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx, "error", err)
				if err != nil {
					return err
				}
				return nil
			case status := <-statusCh:
				log.Infow("container exited", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx, "status", status.StatusCode)
				return nil
			case <-runGroupCtx.Done(): // race with the group
				log.Infow("container group exited", runGroupCtx.Err())
				return nil
			}
		}
		runGroup.Go(f)
	}

	// When we're here, our containers are started, the outcomes are being collected.
	// We wait until either:
	// - all container are done and outcome have been received
	// - we reach a timeout.

	containersAreCompleteCh := make(chan bool)
	outcomesCollectTimeout := make(chan bool)

	// Wait for the containers
	go func() {
		err = runGroup.Wait()
		containersAreCompleteCh <- true
	}()

	startOutcomesCollectTimeout := func() {
		time.Sleep(cfg.OutcomesCollectionTimeout)
		outcomesCollectTimeout <- true
	}

	waitingForContainers := true
	waitingForOutcomes := true

	log.Info("Containers started, waiting for containers and outcome signals")
	for waitingForContainers || waitingForOutcomes {
		select {
		case <-containersAreCompleteCh:
			log.Infow("all containers are complete")
			waitingForContainers = false
			go startOutcomesCollectTimeout()
		case <-outcomesCollectIsCompleteCh:
			log.Infow("all outcomes are complete")
			waitingForOutcomes = false
		case <-outcomesCollectTimeout:
			log.Infow("we timeout'd waiting for outcomes")
			waitingForOutcomes = false
		case <-runCtx.Done():
			log.Infow("the test run ended early")
			return
		}
	}

	return
}

func newDataNetwork(ctx context.Context, cli *client.Client, rw *rpc.OutputWriter, env *api.RunInput, name string) (id string, subnet *net.IPNet, err error) {
	// Find a free network.
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.name=default",
			),
		),
	})
	if err != nil {
		return "", nil, err
	}

	subnet, gateway, err := nextDataNetwork(len(networks))
	if err != nil {
		return "", nil, err
	}

	id, err = docker.NewBridgeNetwork(
		ctx,
		cli,
		fmt.Sprintf("tg-%s-%s-%s-%s", env.TestPlan, env.TestCase, env.RunID, name),
		true,
		map[string]string{
			"testground.plan":     env.TestPlan,
			"testground.testcase": env.TestCase,
			"testground.run_id":   env.RunID,
			"testground.name":     name,
		},
		network.IPAMConfig{
			Subnet:  subnet.String(),
			Gateway: gateway,
		},
	)

	return id, subnet, err
}

func (r *LocalDockerRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	r.lk.RLock()
	dir := r.outputsDir
	r.lk.RUnlock()

	return gzipRunOutputs(ctx, dir, input, ow)
}

// attachContainerToNetwork attaches the provided container to the specified
// network.
func attachContainerToNetwork(ctx context.Context, cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return cli.NetworkConnect(ctx, networkID, containerID, nil)
}

// nolint this function is unused, but it may come in handy.
func detachContainerFromNetwork(ctx context.Context, cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return cli.NetworkDisconnect(ctx, networkID, containerID, true)
}

func (*LocalDockerRunner) ID() string {
	return "local:docker"
}

func (*LocalDockerRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(LocalDockerRunnerConfig{})
}

func (*LocalDockerRunner) CompatibleBuilders() []string {
	return []string{"docker:go", "docker:node", "docker:generic"}
}

// This method deletes the testground containers.
// It does *not* delete any downloaded images or networks.
// I'll leave a friendly message for how to do a more complete cleanup.
func (*LocalDockerRunner) TerminateAll(ctx context.Context, ow *rpc.OutputWriter) error {
	ow.Info("terminate local:docker requested")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// Build two separate queries: one for infrastructure containers, another
	// for test plan containers. The former, we match by container name. The
	// latter, we match by the `testground.purpose` label, which we apply to all
	// plan containers managed by testground label.

	// Build query for runner infrastructure containers.
	infraOpts := types.ContainerListOptions{}
	infraOpts.Filters = filters.NewArgs()
	infraOpts.Filters.Add("name", "testground-grafana")
	infraOpts.Filters.Add("name", "testground-goproxy")
	infraOpts.Filters.Add("name", "testground-influxdb")
	infraOpts.Filters.Add("name", "testground-redis")
	infraOpts.Filters.Add("name", "testground-sidecar")

	// Build query for testground plans that are still running.
	planOpts := types.ContainerListOptions{}
	planOpts.Filters = filters.NewArgs()
	planOpts.Filters.Add("label", "testground.purpose=plan")

	infracontainers, err := cli.ContainerList(ctx, infraOpts)
	if err != nil {
		return fmt.Errorf("failed to list infrastructure containers: %w", err)
	}
	plancontainers, err := cli.ContainerList(ctx, planOpts)
	if err != nil {
		return fmt.Errorf("failed to list test plan containers: %w", err)
	}

	containers := make([]string, 0, len(infracontainers)+len(plancontainers))
	for _, container := range infracontainers {
		containers = append(containers, container.ID)
	}
	for _, container := range plancontainers {
		containers = append(containers, container.ID)
	}

	err = docker.DeleteContainers(cli, ow, containers)
	if err != nil {
		return fmt.Errorf("failed to list testground containers: %w", err)
	}

	ow.Info("to delete networks and images, you may want to run `docker system prune`")
	return nil
}

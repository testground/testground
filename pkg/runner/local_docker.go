package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/testground/sdk-go/ptypes"
	ss "github.com/testground/sdk-go/sync"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"

	"github.com/testground/sdk-go/runtime"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/conv"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/logging"
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

	ExposedPorts ExposedPorts `toml:"exposed_ports"`
}

// defaultConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultConfig = LocalDockerRunnerConfig{
	KeepContainers: false,
	Unstarted:      false,
	Background:     false,
	Ulimits:        []string{"nofile=1048576:1048576"},
	ExposedPorts:   map[string]string{"pprof": "6060"},
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

	syncClient *ss.WatchClient
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

	sidecarContainerOpts := docker.EnsureContainerOpts{
		ContainerName: "testground-sidecar",
		ContainerConfig: &container.Config{
			Image:      "iptestground/sidecar:edge",
			Entrypoint: []string{"testground"},
			Cmd:        []string{"sidecar", "--runner", "docker"},
			Env:        []string{"REDIS_HOST=testground-redis", "INFLUXDB_HOST=testground-influxdb", "GODEBUG=gctrace=1"},
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

func (c *LocalDockerRunner) updateRunResult(template *runtime.RunParams, result *Result) error {
	events, err := c.syncClient.FetchAllEvents(template)
	if err != nil {
		return err
	}

	for _, e := range events {
		// for now we emit only outcome OK events, so no need for more checks
		if e.SuccessEvent != nil {
			se := e.SuccessEvent
			o := result.Outcomes[se.TestGroupID]
			o.Ok = o.Ok + 1
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

	return nil
}

func (r *LocalDockerRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (runoutput *api.RunOutput, err error) {
	// Grab a read lock. This will allow many runs to run simultaneously, but
	// they will be exclusive of state-altering healthchecks.
	r.lk.RLock()
	defer r.lk.RUnlock()

	result := newResult()
	runoutput = &api.RunOutput{
		RunID:  input.RunID,
		Result: result,
	}

	log := ow.With("runner", "local:docker", "run_id", input.RunID)

	r.syncClient, err = ss.NewWatchClient(context.Background(), logging.S())
	if err != nil {
		log.Error(err)
		return
	}

	defer func() {
		if ctx.Err() == context.Canceled {
			result.Outcome = task.OutcomeCanceled
		}
	}()

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}

	// Build a template runenv.
	template := runtime.RunParams{
		TestPlan:          input.TestPlan,
		TestCase:          input.TestCase,
		TestRun:           input.RunID,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
		TestOutputsPath:   "/outputs",
		TestStartTime:     time.Now(),
	}

	// Create a data network.
	dataNetworkID, subnet, err := newDataNetwork(ctx, cli, ow, &template, "default")
	if err != nil {
		return
	}

	template.TestSubnet = &ptypes.IPNet{IPNet: *subnet}

	// Merge the incoming configuration with the default configuration.
	cfg := defaultConfig
	if err = mergo.Merge(&cfg, input.RunnerConfig, mergo.WithOverride); err != nil {
		err = fmt.Errorf("error while merging configurations: %w", err)
		return
	}

	ports := make(nat.PortSet)
	for _, p := range cfg.ExposedPorts {
		ports[nat.Port(p)] = struct{}{}
	}

	type testContainer struct {
		containerID string
		groupID     string
		groupIdx    int
	}

	var containers []testContainer
	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestGroupID = g.ID
		runenv.TestInstanceParams = g.Parameters
		runenv.TestCaptureProfiles = g.Profiles

		result.Outcomes[g.ID] = &GroupOutcome{
			Total: g.Instances,
		}

		reviewResources(g, ow)

		// Serialize the runenv into env variables to pass to docker.
		env := conv.ToOptionsSlice(runenv.ToEnvVars())
		env = append(env, "INFLUXDB_URL=http://testground-influxdb:8086")
		env = append(env, "REDIS_HOST=testground-redis")

		// Inject exposed ports.
		env = append(env, conv.ToOptionsSlice(cfg.ExposedPorts.ToEnvVars())...)

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, "LOG_LEVEL="+cfg.LogLevel)
		}

		// Start as many containers as group instances.
		for i := 0; i < g.Instances; i++ {
			// <outputs_dir>/<plan>/<run_id>/<group_id>/<instance_number>
			odir := filepath.Join(r.outputsDir, input.TestPlan, input.RunID, g.ID, strconv.Itoa(i))
			err = os.MkdirAll(odir, 0777)
			if err != nil {
				err = fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
				break
			}

			name := fmt.Sprintf("tg-%s-%s-%s-%s-%d", input.TestPlan, input.TestCase, input.RunID, g.ID, i)
			log.Infow("creating container", "name", name)

			ccfg := &container.Config{
				Image:        g.ArtifactPath,
				ExposedPorts: ports,
				Env:          env,
				Labels: map[string]string{
					"testground.purpose":  "plan",
					"testground.plan":     input.TestPlan,
					"testground.testcase": input.TestCase,
					"testground.run_id":   input.RunID,
					"testground.group_id": g.ID,
				},
			}

			hcfg := &container.HostConfig{
				NetworkMode:     container.NetworkMode("testground-control"),
				PublishAllPorts: true,
				Mounts: []mount.Mount{{
					Type:   mount.TypeBind,
					Source: odir,
					Target: runenv.TestOutputsPath,
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

			containers = append(containers, testContainer{res.ID, g.ID, i})

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
			_ = docker.DeleteContainers(cli, log, ids)
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

	var (
		doneCh    = make(chan error, 2)
		started   = make(chan testContainer, len(containers))
		ratelimit = make(chan struct{}, 16)
	)

	ctxContainers, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Infow("starting containers", "count", len(containers))

	g, gctx := errgroup.WithContext(ctxContainers)
	for _, c := range containers {
		c := c
		f := func() error {
			ratelimit <- struct{}{}
			defer func() { <-ratelimit }()

			log.Infow("starting container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)

			err := cli.ContainerStart(ctx, c.containerID, types.ContainerStartOptions{})
			if err == nil {
				log.Debugw("started container", "id", c.containerID, "group", c.groupID, "group_index", c.groupIdx)
				select {
				case <-gctx.Done():
				default:
					started <- c
				}
			}
			return err
		}
		g.Go(f)
	}

	// Wait until we're done to close the started channel.
	go func() {
		err := g.Wait()
		close(started)

		if err != nil {
			log.Error(err)
			doneCh <- err
		} else {
			log.Infow("started containers", "count", len(containers))
		}
	}()

	if !cfg.Background {
		pretty := NewPrettyPrinter(ow)

		// This goroutine tails the sidecar container logs and appends them to the pretty printer.
		go func() {
			t := time.Now().Add(time.Duration(-10) * time.Second) // sidecar is a long running daemon, so we care only about logs around the execution of our test run
			stream, err := cli.ContainerLogs(ctx, "testground-sidecar", types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: false,
				Since:      t.Format("2006-01-02T15:04:05"),
				Follow:     true,
			})

			if err != nil {
				doneCh <- err
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

		// This goroutine takes started containers and attaches them to the pretty printer.
		go func() {
		Outer:
			for {
				select {
				case tc, more := <-started:
					if !more {
						break Outer
					}

					stream, err := cli.ContainerLogs(ctx, tc.containerID, types.ContainerLogsOptions{
						ShowStdout: true,
						ShowStderr: true,
						Since:      "2019-01-01T00:00:00",
						Follow:     true,
					})

					if err != nil {
						doneCh <- err
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
					tag := fmt.Sprintf("%s[%03d] (%s)", tc.groupID, tc.groupIdx, tc.containerID[0:6])
					pretty.Manage(tag, rstdout, rstderr)

				case <-ctx.Done():
					// yield if we're been cancelled.
					doneCh <- ctx.Err()
					return
				}
			}

			select {
			case err := <-pretty.Wait():
				doneCh <- err
			case <-ctx.Done():
				log.Error(ctx) // yield if we're been cancelled.
				doneCh <- ctx.Err()
			}
		}()
	}

	select {
	case err = <-doneCh:
		erru := r.updateRunResult(&template, result)
		if erru != nil {
			ow.Errorw("could not update run result", "err", erru)
		}
	case <-ctx.Done():
		err = ctx.Err()
	}

	return
}

func newDataNetwork(ctx context.Context, cli *client.Client, rw *rpc.OutputWriter, env *runtime.RunParams, name string) (id string, subnet *net.IPNet, err error) {
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
		fmt.Sprintf("tg-%s-%s-%s-%s", env.TestPlan, env.TestCase, env.TestRun, name),
		true,
		map[string]string{
			"testground.plan":     env.TestPlan,
			"testground.testcase": env.TestCase,
			"testground.run_id":   env.TestRun,
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

//nolint this function is unused, but it may come in handy.
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

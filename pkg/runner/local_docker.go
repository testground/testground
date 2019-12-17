package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/hashicorp/go-multierror"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	_ api.Runner = &LocalDockerRunner{}
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
	// log messages (default: true).
	Background bool `toml:"background"`
}

// defaultConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultConfig = LocalDockerRunnerConfig{
	KeepContainers: false,
	Unstarted:      false,
	Background:     false,
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
type LocalDockerRunner struct{}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (*LocalDockerRunner) Run(input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		image = input.ArtifactPath
		seq   = input.Seq
		log   = logging.S().With("runner", "local:docker", "run_id", input.RunID)
	)

	var (
		cfg   = defaultConfig
		incfg = input.RunnerConfig.(*LocalDockerRunnerConfig)
	)

	// Merge the incoming configuration with the default configuration.
	if err := mergo.Merge(&cfg, incfg, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("error while merging configurations: %w", err)
	}

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
	}

	// Serialize the runenv into env variables to pass to docker.
	env := util.ToOptionsSlice(runenv.ToEnvVars())

	// Set the log level if provided in cfg.
	if cfg.LogLevel != "" {
		env = append(env, "LOG_LEVEL="+cfg.LogLevel)
	}

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	dataNetworkID, err := newDataNetwork(cli, logging.S(), runenv, "default")
	if err != nil {
		return nil, err
	}
	// Unless we're keeping the containers, delete the network when we're done.
	if !cfg.KeepContainers {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := cli.NetworkRemove(ctx, dataNetworkID); err != nil {
				log.Errorw("removing network", "network", dataNetworkID, "error", err)
			}
		}()
	}

	// Ensure we have a control network.
	controlNetworkID, err := ensureControlNetwork(cli, log)
	if err != nil {
		return nil, err
	}

	// Ensure that we have a testground-redis container; if not, create it.
	_, err = ensureRedisContainer(cli, log, controlNetworkID)
	if err != nil {
		return nil, err
	}

	// Ensure that we have a testground-sidecar container; if not, we'll
	// create it.
	_, err = ensureSidecarContainer(cli, log, controlNetworkID)
	if err != nil {
		if err.Error() == "image not found" {
			newErr := errors.New("Docker image ipfs/testground not found, run `make docker-ipfs-testground`")
			return nil, newErr
		}
		return nil, err
	}

	// Start as many containers as test case instances.
	var containers []string
	for i := 0; i < input.Instances; i++ {
		name := fmt.Sprintf("tg-%s-%s-%s-%d", input.TestPlan.Name, testcase.Name, input.RunID, i)

		log.Infow("creating container", "name", name)

		ccfg := &container.Config{
			Image: image,
			Env:   env,
			Labels: map[string]string{
				"testground.plan":     input.TestPlan.Name,
				"testground.testcase": testcase.Name,
				"testground.runid":    input.RunID,
			},
		}
		hcfg := &container.HostConfig{
			NetworkMode: container.NetworkMode(controlNetworkID),
		}

		// Create the container.
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		res, err := cli.ContainerCreate(ctx, ccfg, hcfg, nil, name)
		cancel()

		if err != nil {
			break
		}

		containers = append(containers, res.ID)

		// TODO: Remove this when we get the sidecar working. It'll do this for us.
		err = attachContainerToNetwork(cli, res.ID, dataNetworkID)
		if err != nil {
			break
		}
	}

	if !cfg.KeepContainers {
		defer deleteContainers(cli, log, containers) //nolint
		// ^^ nolint: this method already logs errors; this is a cleanup action,
		// if an error is returned, there's nothing we can do anyway.
	}

	// If an error occurred interim, delete all containers, and abort.
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// Start the containers, unless the NoStart option is specified.
	if !cfg.Unstarted {
		log.Infow("starting containers", "count", len(containers))

		g, ctx := errgroup.WithContext(context.Background())
		for _, id := range containers {
			if err != nil {
				break
			}
			g.Go(func(id string) func() error {
				return func() error {
					log.Debugw("starting container", "id", id)
					err := cli.ContainerStart(ctx, id, types.ContainerStartOptions{})
					if err == nil {
						log.Debugw("started container", "id", id)
					}
					return err
				}
			}(id))
		}

		// If an error occurred, delete all containers, and abort.
		if err := g.Wait(); err != nil {
			log.Error(err)
			return nil, deleteContainers(cli, log, containers)
		}

		log.Infow("started containers", "count", len(containers))
	}

	if !cfg.Background {
		output := NewConsoleOutput()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for _, id := range containers {
			stream, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Since:      "2019-01-01T00:00:00",
				Follow:     true,
			})

			if err != nil {
				log.Error(err)
				return nil, deleteContainers(cli, log, containers)
			}

			rstdout, wstdout := io.Pipe()
			rstderr, wstderr := io.Pipe()
			go func() {
				_, err := stdcopy.StdCopy(wstdout, wstderr, stream)
				_ = wstdout.CloseWithError(err)
				_ = wstderr.CloseWithError(err)
			}()

			output.Manage(id[0:12], rstdout, rstderr)
		}
		return nil, output.Wait()
	}

	return nil, nil
}

func deleteContainers(cli *client.Client, log *zap.SugaredLogger, ids []string) (err error) {
	log.Infow("deleting containers", "ids", ids)

	errs := make(chan error)
	for _, id := range ids {
		go func(id string) {
			log.Debugw("deleting container", "id", id)
			errs <- cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
		}(id)
	}

	var merr *multierror.Error
	for i := 0; i < len(ids); i++ {
		if err := <-errs; err != nil {
			log.Errorw("failed while deleting container", "error", err)
			merr = multierror.Append(merr, <-errs)
		}
	}
	close(errs)
	return merr.ErrorOrNil()
}

func ensureControlNetwork(cli *client.Client, log *zap.SugaredLogger) (id string, err error) {
	return docker.EnsureBridgeNetwork(context.Background(), log, cli, "testground-control", true)
}

func newDataNetwork(cli *client.Client, log *zap.SugaredLogger, env *runtime.RunEnv, name string) (id string, err error) {
	return docker.NewBridgeNetwork(
		context.Background(),
		cli,
		fmt.Sprintf("tg-%s-%s-%s-%s", env.TestPlan, env.TestCase, env.TestRun, name),
		true,
		map[string]string{
			"testground.plan":     env.TestPlan,
			"testground.testcase": env.TestCase,
			"testground.runid":    env.TestRun,
			"testground.name":     name,
		},
	)
}

// ensureRedisContainer ensures there's a testground-redis container started.
func ensureRedisContainer(cli *client.Client, log *zap.SugaredLogger, controlNetworkID string) (id string, err error) {
	container, _, err := docker.EnsureContainer(context.Background(), log, cli, &docker.EnsureContainerOpts{
		ContainerName: "testground-redis",
		ContainerConfig: &container.Config{
			Image:      "redis",
			Entrypoint: []string{"redis-server"},
			Cmd:        []string{"--notify-keyspace-events", "$szxK"},
		},
		HostConfig: &container.HostConfig{
			NetworkMode: container.NetworkMode(controlNetworkID),
		},
		PullImageIfMissing: true,
	})
	if err != nil {
		return "", err
	}

	return container.ID, err
}

// ensureSidecarContainer ensures there's a testground-sidecar container started.
func ensureSidecarContainer(cli *client.Client, log *zap.SugaredLogger, controlNetworkID string) (id string, err error) {
	container, _, err := docker.EnsureContainer(context.Background(), log, cli, &docker.EnsureContainerOpts{
		ContainerName: "testground-sidecar",
		ContainerConfig: &container.Config{
			Image:      "ipfs/testground:latest",
			Entrypoint: []string{"testground"},
			Cmd:        []string{"sidecar", "--runner", "docker"},
			Env:        []string{"REDIS_HOST=testground-redis"},
		},
		HostConfig: &container.HostConfig{
			NetworkMode: container.NetworkMode(controlNetworkID),
			// To lookup namespaces. Can't use SandboxKey for some reason.
			PidMode: "host",
			// We need _both_ to actually get a network namespace handle.
			// We may be able to drop sys_admin if we drop netlink
			// sockets that we're not using.
			CapAdd: []string{"NET_ADMIN", "SYS_ADMIN"},
			// needed to talk to docker.
			Mounts: []mount.Mount{{
				Type:   mount.TypeBind,
				Source: "/var/run/docker.sock", // TODO: don't hardcode this.
				Target: "/var/run/docker.sock",
			}},
		},
		PullImageIfMissing: false, // Don't pull from Docker Hub
	})
	if err != nil {
		return "", err
	}

	return container.ID, err
}

// attachContainerToNetwork attaches the provided container to the specified
// network.
func attachContainerToNetwork(cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return cli.NetworkConnect(ctx, networkID, containerID, nil)
}

//nolint this function is unused, but it may come in handy.
func detachContainerFromNetwork(cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	return []string{"docker:go"}
}

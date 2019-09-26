package runner

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/google/uuid"
	"github.com/imdario/mergo"
)

var (
	_ Runner = &LocalDockerRunner{}
)

type LocalDockerRunnerConfig struct {
	// Enabled indicates if this runner is enabled.
	Enabled bool
	// RmContainers removes test containers as soon as they exit (default: true).
	RmContainers bool `toml:"rm"`
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`
	// NoStart only creates the containers without starting them (default: false).
	NoStart bool `toml:"no_start"`
	// Tail tails the output of all containers and logs as info messages (default: true).
	Tail bool `toml:"tail"`
}

// defaultConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultConfig = LocalDockerRunnerConfig{
	Enabled:      true,
	RmContainers: true,
	NoStart:      false,
	Tail:         true,
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
func (*LocalDockerRunner) Run(input *Input) (*Output, error) {
	var (
		image    = input.ArtifactPath
		seq      = input.Seq
		deferred []func() error
		log      = logging.S().With("runner", "local:docker")
	)

	defer func() {
		for i := len(deferred) - 1; i >= 0; i-- {
			if err := deferred[i](); err != nil {
				log.Errorw("error while calling deferred functions", "error", err)
			}
		}
	}()

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
		TestRun:            input.ID,
		TestCaseSeq:        seq,
		TestInstanceCount:  input.Instances,
		TestInstanceParams: input.Parameters,
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

	// Create a user-defined bridge. We will attach the redis container, as well
	// as all test containers to it.
	networkID, err := newUserDefinedBridge(cli)
	if err != nil {
		return nil, err
	}

	// Ensure that we have a testground-redis container; if not, we'll create
	// it.
	redisContainerID, err := ensureRedisContainer(cli, log)
	if err != nil {
		return nil, err
	}

	// Attach the testground-redis container to the user-defined bridge.
	_, err = attachContainerToNetwork(cli, redisContainerID, networkID)
	if err != nil {
		return nil, err
	}
	// deferred = append(deferred, detach)

	// Start as many containers as test case instances.
	var containers []string
	for i := 0; i < input.Instances; i++ {
		name := fmt.Sprintf("tg-%s-%s-%s-%d", input.TestPlan.Name, testcase.Name, input.ID, i)

		log.Infow("creating container", "name", name)

		ccfg := &container.Config{
			Image: image,
			Env:   env,
		}
		hcfg := &container.HostConfig{
			NetworkMode: container.NetworkMode(networkID),
			AutoRemove:  cfg.RmContainers,
		}

		// Create the container.
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		res, err := cli.ContainerCreate(ctx, ccfg, hcfg, nil, name)
		cancel()

		if err != nil {
			break
		}
		containers = append(containers, res.ID)
	}

	// If an error occurred interim, delete all containers, and abort.
	if err != nil {
		log.Error(err)
		log.Infow("deleting containers", "ids", containers)
		if err := deleteContainers(cli, log, containers); err != nil {
			log.Errorw("failed while deleting containers", "error", err)
			return nil, err
		}
		return nil, err
	}

	return nil, nil
}

func deleteContainers(cli *client.Client, log *zap.SugaredLogger, containerIDs []string) (err error) {
	for _, id := range containerIDs {
		if err != nil {
			break
		}

		log.Debugw("deleting container", "id", id)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
			Force: true,
		})
		cancel()
	}
	return err
}

func newUserDefinedBridge(cli *client.Client) (id string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	res, err := cli.NetworkCreate(ctx, uuid.String(), types.NetworkCreate{
		Driver:     "bridge",
		Attachable: true,
	})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

// ensureRedisContainer ensures there's a testground-redis container started.
func ensureRedisContainer(cli *client.Client, log *zap.SugaredLogger) (id string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Debug("checking state of redis container")

	// Check if a testground-redis container exists.
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", "testground-redis")),
	})
	if err != nil {
		return "", err
	}

	if len(containers) > 0 {
		container := containers[0]

		log.Infow("redis container found",
			"containerId", container.ID, "state", container.State)

		switch container.State {
		case "running":
		default:
			ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
			defer func(fn context.CancelFunc) { fn() }(cancel)

			log.Infof("redis container isn't running; starting")

			err := cli.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
			if err != nil {
				log.Errorf("starting redis container failed", "containerId", container.ID)
				return "", err
			}
		}
		return container.ID, nil
	}

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	res, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      "redis",
		Entrypoint: []string{"redis-server"},
		Cmd:        []string{"--notify-keyspace-events", "$szxK"},
	}, nil, nil, "testground-redis")

	return res.ID, err
}

// attachContainerToNetwork attaches the provided container to the specified
// network, returning a callback function that dissolves the attachment.
func attachContainerToNetwork(cli *client.Client, containerID string, networkID string) (func() error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cli.NetworkConnect(ctx, networkID, containerID, nil); err != nil {
		return nil, err
	}
	discFn := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		return cli.NetworkDisconnect(ctx, networkID, containerID, true)
	}
	return discFn, nil
}

func (*LocalDockerRunner) OverridableParameters() []string {
	return nil
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

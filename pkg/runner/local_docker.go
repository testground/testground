package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/docker/docker/pkg/stdcopy"

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
	"github.com/logrusorgru/aurora"
)

var (
	_ Runner = &LocalDockerRunner{}
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
	Background bool `toml:"tail"`
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
func (*LocalDockerRunner) Run(input *Input) (*Output, error) {
	var (
		image    = input.ArtifactPath
		seq      = input.Seq
		deferred []func() error
		log      = logging.S().With("runner", "local:docker", "run_id", input.ID)
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
			AutoRemove:  !cfg.KeepContainers,
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
		return nil, deleteContainers(cli, log, containers)
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

	type containerReader struct {
		io.ReadCloser
		id    string
		color uint8
	}

	a := aurora.NewAurora(logging.IsTerminal())

	if !cfg.Background {
		var wg sync.WaitGroup
		for n, id := range containers {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			stream, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Since:      "2019-01-02T00:00:00",
				Follow:     true,
			})
			defer cancel()

			if err != nil {
				log.Error(err)
				return nil, deleteContainers(cli, log, containers)
			}

			rpipe, wpipe := io.Pipe()
			reader := containerReader{
				ReadCloser: rpipe,
				id:         id,
				color:      uint8(n%15) + 1,
			}

			go func() {
				_, err := stdcopy.StdCopy(wpipe, wpipe, stream)
				wpipe.CloseWithError(err)
			}()

			wg.Add(1)
			go func(r *containerReader) {
				defer wg.Done()
				defer r.Close()

				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					fmt.Println(a.Index(r.color, "<< "+r.id+" >>"), scanner.Text())
				}
			}(&reader)
		}
		wg.Wait()
	}

	return nil, nil
}

func deleteContainers(cli *client.Client, log *zap.SugaredLogger, ids []string) (err error) {
	log.Infow("deleting containers", "ids", ids)
	for _, id := range ids {
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

	if err != nil {
		log.Errorw("failed while deleting containers", "error", err)
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

	log.Infow("redis container not found; creating")

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	res, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      "redis",
		Entrypoint: []string{"redis-server"},
		Cmd:        []string{"--notify-keyspace-events", "$szxK"},
	}, nil, nil, "testground-redis")

	if err != nil {
		return "", err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	log.Infow("starting new redis container", "id", res.ID)

	err = cli.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	if err == nil {
		log.Infow("started redis container", "id", res.ID)
	}

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

func (*LocalDockerRunner) ID() string {
	return "local:docker"
}

func (*LocalDockerRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(LocalDockerRunnerConfig{})
}

func (*LocalDockerRunner) CompatibleBuilders() []string {
	return []string{"docker:go"}
}

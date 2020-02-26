package runner

import (
	"context"
	"errors"
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

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
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
	// log messages (default: false).
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
type LocalDockerRunner struct {
	setupLk sync.Mutex
}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (r *LocalDockerRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		seq = input.Seq
		log = logging.S().With("runner", "local:docker", "run_id", input.RunID)
		err error
	)

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	testcase := input.TestPlan.TestCases[seq]

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	r.setupLk.Lock()

	// Ensure we have a control network.
	ctrlnid, err := ensureControlNetwork(ctx, cli, log)
	if err != nil {
		r.setupLk.Unlock()
		return nil, err
	}

	// Ensure that we have a testground-redis container; if not, create it.
	_, err = ensureRedisContainer(ctx, cli, log, ctrlnid)
	if err != nil {
		r.setupLk.Unlock()
		return nil, err
	}

	// Ensure the outputs dir exists.
	outputsDir := filepath.Join(input.EnvConfig.WorkDir(), "local_docker", "outputs")
	if err := os.MkdirAll(outputsDir, 0777); err != nil {
		r.setupLk.Unlock()
		return nil, err
	}

	// Ensure that we have a testground-sidecar container; if not, we'll
	// create it.
	switch _, err = ensureSidecarContainer(ctx, cli, outputsDir, log, ctrlnid); err {
	case nil:
	case errors.New("image not found"):
		r.setupLk.Unlock()
		return nil, errors.New("Docker image ipfs/testground not found, run `make docker-ipfs-testground`")
	default:
		r.setupLk.Unlock()
		return nil, err
	}

	r.setupLk.Unlock()

	// Build a template runenv.
	template := runtime.RunParams{
		TestPlan:          input.TestPlan.Name,
		TestCase:          testcase.Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
		TestOutputsPath:   "/outputs",
		TestStartTime:     time.Now(),
	}

	// Create a data network.
	dataNetworkID, subnet, err := newDataNetwork(ctx, cli, logging.S(), &template, "default")
	if err != nil {
		return nil, err
	}

	template.TestSubnet = &runtime.IPNet{IPNet: *subnet}

	// Merge the incoming configuration with the default configuration.
	cfg := defaultConfig
	if err := mergo.Merge(&cfg, input.RunnerConfig, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("error while merging configurations: %w", err)
	}

	var containers []string
	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestGroupID = g.ID
		runenv.TestInstanceParams = g.Parameters

		// Serialize the runenv into env variables to pass to docker.
		env := conv.ToOptionsSlice(runenv.ToEnvVars())

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, "LOG_LEVEL="+cfg.LogLevel)
		}

		// Create the run output directory and write the runenv.
		runDir := filepath.Join(outputsDir, input.TestPlan.Name, input.RunID, g.ID)
		if err := os.MkdirAll(runDir, 0777); err != nil {
			return nil, err
		}

		// Start as many containers as group instances.
		for i := 0; i < g.Instances; i++ {
			// <outputs_dir>/<plan>/<run_id>/<group_id>/<instance_number>
			odir := filepath.Join(outputsDir, input.TestPlan.Name, input.RunID, g.ID, strconv.Itoa(i))
			err = os.MkdirAll(odir, 0777)
			if err != nil {
				err = fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
				break
			}

			name := fmt.Sprintf("tg-%s-%s-%s-%s-%d", input.TestPlan.Name, testcase.Name, input.RunID, g.ID, i)
			log.Infow("creating container", "name", name)

			ccfg := &container.Config{
				Image: g.ArtifactPath,
				Env:   env,
				Labels: map[string]string{
					"testground.plan":     input.TestPlan.Name,
					"testground.testcase": testcase.Name,
					"testground.run_id":   input.RunID,
					"testground.group_id": g.ID,
				},
			}
			fmt.Println(odir)
			hcfg := &container.HostConfig{
				NetworkMode: container.NetworkMode(ctrlnid),
				Mounts: []mount.Mount{{
					Type:   mount.TypeBind,
					Source: odir,
					Target: runenv.TestOutputsPath,
				}},
			}

			// Create the container.
			var res container.ContainerCreateCreatedBody
			res, err = cli.ContainerCreate(ctx, ccfg, hcfg, nil, name)
			if err != nil {
				break
			}

			containers = append(containers, res.ID)

			// TODO: Remove this when we get the sidecar working. It'll do this for us.
			err = attachContainerToNetwork(ctx, cli, res.ID, dataNetworkID)
			if err != nil {
				break
			}
		}
	}

	if !cfg.KeepContainers {
		defer func() {
			_ = deleteContainers(cli, log, containers)
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
		return nil, err
	}

	// Start the containers.
	if !cfg.Unstarted {
		log.Infow("starting containers", "count", len(containers))
		g, ctx := errgroup.WithContext(ctx)

		for _, id := range containers {
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
		pretty := NewPrettyPrinter()

		ctx, cancel := context.WithCancel(ctx)
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

			pretty.Manage(id[0:12], rstdout, rstderr)
		}
		return &api.RunOutput{RunID: input.RunID}, pretty.Wait()
	}

	return &api.RunOutput{RunID: input.RunID}, nil
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

func ensureControlNetwork(ctx context.Context, cli *client.Client, log *zap.SugaredLogger) (id string, err error) {
	return docker.EnsureBridgeNetwork(
		ctx,
		log, cli,
		"testground-control",
		true,
		network.IPAMConfig{
			Subnet:  controlSubnet,
			Gateway: controlGateway,
		},
	)
}

func newDataNetwork(ctx context.Context, cli *client.Client, log *zap.SugaredLogger, env *runtime.RunParams, name string) (id string, subnet *net.IPNet, err error) {
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

// ensureRedisContainer ensures there's a testground-redis container started.
func ensureRedisContainer(ctx context.Context, cli *client.Client, log *zap.SugaredLogger, controlNetworkID string) (id string, err error) {
	container, _, err := docker.EnsureContainer(ctx, log, cli, &docker.EnsureContainerOpts{
		ContainerName: "testground-redis",
		ContainerConfig: &container.Config{
			Image:      "redis",
			Entrypoint: []string{"redis-server"},
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
func ensureSidecarContainer(ctx context.Context, cli *client.Client, workDir string, log *zap.SugaredLogger, controlNetworkID string) (id string, err error) {
	dockerSock := "/var/run/docker.sock"
	if host := cli.DaemonHost(); strings.HasPrefix(host, "unix://") {
		dockerSock = host[len("unix://"):]
	} else {
		log.Warnf("guessing docker socket as %s", dockerSock)
	}
	container, _, err := docker.EnsureContainer(ctx, log, cli, &docker.EnsureContainerOpts{
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
				Source: dockerSock,
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

func (*LocalDockerRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, w io.Writer) error {
	basedir := filepath.Join(input.EnvConfig.WorkDir(), "local_docker", "outputs")
	return zipRunOutputs(ctx, basedir, input, w)
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
	return []string{"docker:go"}
}

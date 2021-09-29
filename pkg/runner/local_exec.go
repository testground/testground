package runner

import (
	"context"
	"fmt"
	"github.com/testground/sdk-go/ptypes"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/testground/sdk-go/runtime"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/conv"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

var (
	_, localSubnet, _ = net.ParseCIDR("127.1.0.1/16")
)

var (
	_ api.Runner        = (*LocalExecutableRunner)(nil)
	_ api.Healthchecker = (*LocalExecutableRunner)(nil)
)

type LocalExecutableRunner struct {
	lk sync.RWMutex

	outputsDir string
}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (r *LocalExecutableRunner) Healthcheck(ctx context.Context, engine api.Engine, ow *rpc.OutputWriter, fix bool) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	r.outputsDir = filepath.Join(engine.EnvConfig().Dirs().Outputs(), "local_exec")
	hh := &healthcheck.Helper{}

	hh.Enlist("redis-port",
		healthcheck.CheckRedisPort(ctx, ow, cli),
		healthcheck.RequiresManualFixing(),
	)

	// setup infra which is common between local:docker and local:exec
	localCommonHealthcheck(ctx, hh, cli, ow, "testground-control", r.outputsDir)

	// RunChecks will fill the report and return any errors.
	return hh.RunChecks(ctx, fix)
}

func (r *LocalExecutableRunner) Close() error {
	return nil
}

func (r *LocalExecutableRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (*api.RunOutput, error) {
	r.lk.RLock()
	defer r.lk.RUnlock()

	// Build a template runenv.
	template := runtime.RunParams{
		TestPlan:          input.TestPlan,
		TestCase:          input.TestCase,
		TestRun:           input.RunID,
		TestInstanceCount: input.TotalInstances,
		TestDisableInflux: input.DisableInflux,
		TestSidecar:       false,
		TestSubnet:        &ptypes.IPNet{IPNet: *localSubnet},
	}

	// Spawn as many instances as the input parameters require.
	pretty := NewPrettyPrinter(ow)
	commands := make([]*exec.Cmd, 0, input.TotalInstances)
	defer func() {
		for _, cmd := range commands {
			_ = cmd.Process.Kill()
		}
		for _, cmd := range commands {
			_ = cmd.Wait()
		}
		_ = pretty.Wait()
	}()

	var (
		total   int
		tmpdirs []string
	)
	for _, g := range input.Groups {
		reviewResources(g, ow)

		for i := 0; i < g.Instances; i++ {
			total++
			tag := fmt.Sprintf("%s[%03d]", g.ID, i)

			odir := filepath.Join(r.outputsDir, input.TestPlan, input.RunID, g.ID, strconv.Itoa(i))
			if err := os.MkdirAll(odir, 0777); err != nil {
				err = fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
				pretty.FailStart(tag, err)
				continue
			}

			tmpdir, err := ioutil.TempDir("", "testground")
			if err != nil {
				err = fmt.Errorf("failed to create temp dir: %s: %w", tmpdir, err)
				pretty.FailStart(tag, err)
				continue
			}

			tmpdirs = append(tmpdirs, tmpdir)

			runenv := template
			runenv.TestGroupID = g.ID
			runenv.TestGroupInstanceCount = g.Instances
			runenv.TestInstanceParams = g.Parameters
			runenv.TestOutputsPath = odir
			runenv.TestTempPath = tmpdir
			runenv.TestStartTime = time.Now()
			runenv.TestCaptureProfiles = g.Profiles

			env := conv.ToOptionsSlice(runenv.ToEnvVars())
			env = append(env, "INFLUXDB_URL=http://localhost:8086")
			// NOTE: we export REDIS_HOST for compatibility with older sdk versions.
			env = append(env, "REDIS_HOST=localhost")
			env = append(env, "SYNC_SERVICE_HOST=localhost")
			env = append(env, "PATH="+os.Getenv("PATH"))

			ow.Infow("starting test case instance", "plan", input.TestPlan, "group", g.ID, "number", i, "total", total)

			cmd := exec.CommandContext(ctx, g.ArtifactPath)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()
			cmd.Env = env

			if err := cmd.Start(); err != nil {
				pretty.FailStart(tag, err)
				continue
			}

			commands = append(commands, cmd)

			// instance tag in output: << group[zero_padded_i] >>, e.g. << miner[003] >>
			pretty.Manage(tag, stdout, stderr)
		}
	}

	if err := <-pretty.Wait(); err != nil {
		return nil, err
	}

	// remove all temporary directories.
	for _, tmpdir := range tmpdirs {
		_ = os.RemoveAll(tmpdir)
	}

	return &api.RunOutput{RunID: input.RunID}, nil
}

func (r *LocalExecutableRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	r.lk.RLock()
	dir := r.outputsDir
	r.lk.RUnlock()

	return gzipRunOutputs(ctx, dir, input, ow)
}

func (*LocalExecutableRunner) ID() string {
	return "local:exec"
}

func (*LocalExecutableRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(LocalExecutableRunnerCfg{})
}

func (*LocalExecutableRunner) CompatibleBuilders() []string {
	return []string{"exec:go"}
}

func (*LocalExecutableRunner) TerminateAll(ctx context.Context, ow *rpc.OutputWriter) error {
	// TODO: we're only stopping infrastructure/dependency containers.
	//  We are not kill the test plan processes started by this runner.
	//  It's possible that it's entirely unnecessary to do so, because we use
	//  exec.CommandContext, associating the request context.
	//  So assuming the user has cancelled the request context, those processes
	//  should die consequently. However, it's possible that the termination
	//  call is received while a run is inflight.
	//  To cater for that, and also to play it safe, this method should find all
	//  children processes of the daemon, and send them a SIGKILL.
	ow.Info("terminate local:exec requested")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// Build query for runner infrastructure containers.
	opts := types.ContainerListOptions{}
	opts.Filters = filters.NewArgs()
	opts.Filters.Add("name", "testground-grafana")
	opts.Filters.Add("name", "testground-goproxy")
	opts.Filters.Add("name", "testground-redis")
	opts.Filters.Add("name", "testground-influxdb")
	opts.Filters.Add("name", "testground-sidecar")

	infracontainers, err := cli.ContainerList(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list infrastructure containers: %w", err)
	}

	containers := make([]string, 0, len(infracontainers))
	for _, container := range infracontainers {
		containers = append(containers, container.ID)
	}

	err = docker.DeleteContainers(cli, ow, containers)
	if err != nil {
		return fmt.Errorf("failed to list testground containers: %w", err)
	}

	ow.Info("to delete networks and images, you may want to run `docker system prune`")
	return nil
}

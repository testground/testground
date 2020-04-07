package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	hc "github.com/ipfs/testground/pkg/healthcheck"
	"github.com/ipfs/testground/pkg/rpc"
	"github.com/ipfs/testground/sdk/runtime"
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
type LocalExecutableRunnerCfg struct {
	// How to reach influxdb
	InfluxURL    string `toml:"influx_url"`
	InfluxToken  string `toml:"influx_token"`
	InfluxOrg    string `toml:"influx_org"`
	InfluxBucket string `toml:"influx_bucket"`
}

func (r *LocalExecutableRunner) Healthcheck(ctx context.Context, engine api.Engine, ow *rpc.OutputWriter, fix bool) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	r.outputsDir = filepath.Join(engine.EnvConfig().WorkDir(), "local_exec", "outputs")
	srcDir := engine.EnvConfig().SrcDir
	hh := &hc.Helper{}

	// setup infra which is common between local:docker and local:exec
	localCommonHealthcheck(ctx, hh, cli, ow, "testground-control", srcDir, r.outputsDir)

	// RunChecks will fill the report and return any errors.
	return hh.RunChecks(ctx, fix)
}

func (r *LocalExecutableRunner) Close() error {
	return nil
}

func (r *LocalExecutableRunner) Run(ctx context.Context, input *api.RunInput, ow *rpc.OutputWriter) (*api.RunOutput, error) {
	r.lk.RLock()
	defer r.lk.RUnlock()

	var (
		plan = input.TestPlan
		seq  = input.Seq
		name = plan.Name
	)

	cfg := *input.RunnerConfig.(*LocalExecutableRunnerCfg)

	if seq >= len(plan.TestCases) {
		return nil, fmt.Errorf("invalid sequence number %d for test %s", seq, name)
	}

	// Build a template runenv.
	template := runtime.RunParams{
		TestPlan:          input.TestPlan.Name,
		TestCase:          input.TestPlan.TestCases[seq].Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       false,
		TestSubnet:        &runtime.IPNet{IPNet: *localSubnet},
		TestInfluxURL:     cfg.InfluxURL,
		TestInfluxToken:   cfg.InfluxToken,
		TestInfluxOrg:     cfg.InfluxOrg,
		TestInfluxBucket:  cfg.InfluxBucket,
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

	var total int
	for _, g := range input.Groups {
		for i := 0; i < g.Instances; i++ {
			total++
			id := fmt.Sprintf("instance %3d", total)

			odir := filepath.Join(r.outputsDir, input.TestPlan.Name, input.RunID, g.ID, strconv.Itoa(i))
			if err := os.MkdirAll(odir, 0777); err != nil {
				err = fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
				pretty.FailStart(id, err)
				continue
			}

			runenv := template
			runenv.TestGroupID = g.ID
			runenv.TestGroupInstanceCount = g.Instances
			runenv.TestInstanceParams = g.Parameters
			runenv.TestOutputsPath = odir
			runenv.TestStartTime = time.Now()

			env := conv.ToOptionsSlice(runenv.ToEnvVars())

			ow.Infow("starting test case instance", "plan", name, "group", g.ID, "number", i, "total", total)

			cmd := exec.CommandContext(ctx, g.ArtifactPath)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()
			cmd.Env = env

			if err := cmd.Start(); err != nil {
				pretty.FailStart(id, err)
				continue
			}

			commands = append(commands, cmd)

			pretty.Manage(id, stdout, stderr)
		}
	}

	if err := <-pretty.Wait(); err != nil {
		return nil, err
	}

	return &api.RunOutput{RunID: input.RunID}, nil
}

func (*LocalExecutableRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	basedir := filepath.Join(input.EnvConfig.WorkDir(), "local_exec", "outputs")
	return zipRunOutputs(ctx, basedir, input, ow)
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
	opts.Filters.Add("name", "prometheus-pushgateway")
	opts.Filters.Add("name", "testground-grafana")
	opts.Filters.Add("name", "testground-prometheus")
	opts.Filters.Add("name", "testground-redis")
	opts.Filters.Add("name", "testground-sidecar")

	infracontainers, err := cli.ContainerList(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list infrastructure containers: %w", err)
	}

	containers := make([]string, 0, len(infracontainers))
	for _, container := range infracontainers {
		containers = append(containers, container.ID)
	}

	err = deleteContainers(cli, ow, containers)
	if err != nil {
		return fmt.Errorf("failed to list testground containers: %w", err)
	}

	ow.Info("to delete networks and images, you may want to run `docker system prune`")
	return nil
}

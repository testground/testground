package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"net"
	"os/exec"
	"reflect"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	hc "github.com/ipfs/testground/pkg/healthcheck"
	"github.com/ipfs/testground/pkg/logging"
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
	closeFn    context.CancelFunc
}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (r *LocalExecutableRunner) Healthcheck(fix bool, engine api.Engine, ow *rpc.OutputWriter) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	ctx, cancel := context.WithCancel(engine.Context())
	r.closeFn = cancel

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
	if r.closeFn != nil {
		r.closeFn()
		logging.S().Info("temporary redis instance stopped")
	}

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
	infraOpts.Filters.Add("name", "prometheus-pushgateway")
	infraOpts.Filters.Add("name", "testground-goproxy")
	infraOpts.Filters.Add("name", "testground-grafana")
	infraOpts.Filters.Add("name", "testground-prometheus")
	infraOpts.Filters.Add("name", "testground-redis")
	infraOpts.Filters.Add("name", "testground-sidecar")

	infracontainers, err := cli.ContainerList(ctx, infraOpts)
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

package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"io"
	"net"
	"os/exec"
	"reflect"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/logging"
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

func (r *LocalExecutableRunner) Healthcheck(fix bool, engine api.Engine, writer io.Writer) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	ctx, cancel := context.WithCancel(engine.Context())
	r.closeFn = cancel

	log := logging.S().With("runner", "local:docker")

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	r.outputsDir = filepath.Join(engine.EnvConfig().WorkDir(), "local_exec", "outputs")
	report := api.HealthcheckReport{}
	hcHelper := ErrgroupHealthcheckHelper{report: &report}

	// setup infra which is common between local:docker and local:exec
	healthcheck_common_local_infra(&hcHelper, ctx, log, cli, "testground-control", engine.EnvConfig().SrcDir, r.outputsDir)

	// RunChecks will fill the report and return any errors.
	err = hcHelper.RunChecks(ctx, fix)

	return &report, err
}

func (r *LocalExecutableRunner) Close() error {
	if r.closeFn != nil {
		r.closeFn()
		logging.S().Info("temporary redis instance stopped")
	}

	return nil
}

func (r *LocalExecutableRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
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
	pretty := NewPrettyPrinter()
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

			logging.S().Infow("starting test case instance", "plan", name, "group", g.ID, "number", i, "total", total)

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

func (*LocalExecutableRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, w io.Writer) error {
	basedir := filepath.Join(input.EnvConfig.WorkDir(), "local_exec", "outputs")
	return zipRunOutputs(ctx, basedir, input, w)
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

func (*LocalExecutableRunner) TerminateAll(ctx context.Context) error {
	log := logging.S()
	log.Info("terminate local:docker requested")

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

	err = deleteContainers(cli, log, containers)
	if err != nil {
		return fmt.Errorf("failed to list testground containers: %w", err)
	}

	log.Info("to delete networks and images, you may want to run `docker system prune`")
	return nil
}

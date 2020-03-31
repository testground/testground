package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"net"
	"os/exec"
	"reflect"
	"sync"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
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

type healthcheckedProcess struct {
	HealthcheckItem api.HealthcheckItem
	Checker         func() bool
	Fixer           func() error
	Success         string
	Failure         string
}

func newhealthcheckedProcess(ctx context.Context, name string, address string, cmd string, args ...string) *healthcheckedProcess {
	return &healthcheckedProcess{
		HealthcheckItem: api.HealthcheckItem{
			Name: name,
		},
		Checker: tcpChecker(address),
		Fixer:   commandStarter(ctx, cmd, args...),
		Success: fmt.Sprintf("%s instance check: OK", name),
		Failure: fmt.Sprintf("%s instance check: FAIL", name),
	}
}

// tcpChecker returns a closure which can be used to check
// when a tcp port is open. Returns true if the socket is dialable.
// Otherwise, returns false. Use as healthcheckedProcess.Checker.
func tcpChecker(address string) func() bool {
	return func() bool {
		_, err := net.Dial("tcp", address)
		return err == nil
	}
}

func commandStarter(ctx context.Context, cmd string, args ...string) func() error {
	return func() error {
		cmd := exec.CommandContext(ctx, cmd, args...)
		return cmd.Start()
	}
}

func (r *LocalExecutableRunner) Healthcheck(fix bool, engine api.Engine, ow *rpc.OutputWriter) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	ctx, cancel := context.WithCancel(engine.Context())
	r.closeFn = cancel

	report := api.HealthcheckReport{
		Checks: []api.HealthcheckItem{},
		Fixes:  []api.HealthcheckItem{},
	}

	// Use this slice to add or remove additional checks.
	localInfra := []*healthcheckedProcess{
		newhealthcheckedProcess(ctx,
			"local-redis",
			"localhost:6379",
			"redis-server",
			"--save",
			"\"\"",
			"--appendonly",
			"no"),
		newhealthcheckedProcess(ctx,
			"local-prometheus",
			"localhost:9090",
			"prometheus"),
		newhealthcheckedProcess(ctx,
			"local-pushgateway",
			"localhost:9091",
			"pushgateway"),
	}

	eg, _ := errgroup.WithContext(ctx)

	for _, li := range localInfra {
		hcp := *li
		eg.Go(func() error {
			// Checker succeeds, already working.
			if hcp.Checker() {
				hcp.HealthcheckItem.Status = api.HealthcheckStatusOK
				hcp.HealthcheckItem.Message = hcp.Success
				report.Checks = append(report.Checks, hcp.HealthcheckItem)
				return nil
			}
			// Checker failed, try to fix.
			err := hcp.Fixer()
			if err != nil {
				// Oh no! the fix failed.
				hcp.HealthcheckItem.Status = api.HealthcheckStatusFailed
				hcp.HealthcheckItem.Message = hcp.Failure
				report.Checks = append(report.Checks, hcp.HealthcheckItem)
				// just because the fixer failed, doesn't mean *this* procedure failed.
				return nil
			}
			// Fix succeeded.
			hcp.HealthcheckItem.Status = api.HealthcheckStatusOK
			hcp.HealthcheckItem.Message = hcp.Success
			report.Checks = append(report.Checks, hcp.HealthcheckItem)
			report.Fixes = append(report.Fixes, hcp.HealthcheckItem)
			return nil
		})
		err := eg.Wait()
		if err != nil {
			return nil, nil
		}
	}

	var outputsDirCheck api.HealthcheckItem
	// Ensure the outputs dir exists.
	r.outputsDir = filepath.Join(engine.EnvConfig().WorkDir(), "local_exec", "outputs")
	if _, err := os.Stat(r.outputsDir); err == nil {
		msg := "outputs directory exists"
		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusOK, Message: msg}
	} else if os.IsNotExist(err) {
		msg := "outputs directory does not exist"
		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusFailed, Message: msg}
	} else {
		msg := fmt.Sprintf("failed to stat outputs directory: %s", err)
		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusAborted, Message: msg}
	}

	if outputsDirCheck.Status != api.HealthcheckStatusOK {
		if err := os.MkdirAll(r.outputsDir, 0777); err == nil {
			msg := "outputs dir created successfully"
			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusOK, Message: msg}
			report.Fixes = append(report.Fixes, it)
		} else {
			msg := fmt.Sprintf("failed to create outputs dir: %s", err)
			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusFailed, Message: msg}
			report.Fixes = append(report.Fixes, it)
		}
	}

	return &report, nil
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

func (*LocalExecutableRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, ow *rpc.OutputWriter, file io.Writer) error {
	basedir := filepath.Join(input.EnvConfig.WorkDir(), "local_exec", "outputs")
	return zipRunOutputs(ctx, basedir, input, ow, file)
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

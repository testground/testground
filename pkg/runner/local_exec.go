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

	outputsDir   string
	redisCloseFn context.CancelFunc
}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (r *LocalExecutableRunner) Healthcheck(fix bool, engine api.Engine, writer io.Writer) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	var redisCheck, outputsDirCheck api.HealthcheckItem

	// Check if a local Redis instance is running. If not, try to start it.
	_, err := net.Dial("tcp", "localhost:6379")
	if err == nil {
		msg := "local redis instance check: OK"
		redisCheck = api.HealthcheckItem{Name: "local-redis", Status: api.HealthcheckStatusOK, Message: msg}
	} else {
		msg := fmt.Sprintf("local redis instance check: FAIL; %s", err)
		redisCheck = api.HealthcheckItem{Name: "local-redis", Status: api.HealthcheckStatusFailed, Message: msg}
	}

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

	if !fix {
		return &api.HealthcheckReport{Checks: []api.HealthcheckItem{redisCheck, outputsDirCheck}}, nil
	}

	// FIX LOGIC ====================

	var fixes []api.HealthcheckItem

	if redisCheck.Status != api.HealthcheckStatusOK {
		// if there was a previous close function, it's possible that the
		// user has killed the redis instance manually, or some other
		// pathological situation is in place. Call the close function
		// first just in case.
		if r.redisCloseFn != nil {
			r.redisCloseFn()
		}

		// Create a new cancellable context for the redis process. Store the
		// cancel function and renew the sync.Once in the runner's state.
		ctx, cancel := context.WithCancel(engine.Context())
		r.redisCloseFn = cancel

		cmd := exec.CommandContext(ctx, "redis-server", "--save", "\"\"", "--appendonly", "no")
		if err := cmd.Start(); err == nil {
			msg := "temporary redis instance started successfully"
			it := api.HealthcheckItem{Name: "local-redis", Status: api.HealthcheckStatusOK, Message: msg}
			fixes = append(fixes, it)
		} else {
			msg := fmt.Sprintf("temporary redis instance failed to start: %s", err)
			it := api.HealthcheckItem{Name: "local-redis", Status: api.HealthcheckStatusFailed, Message: msg}
			fixes = append(fixes, it)
		}
	}

	if outputsDirCheck.Status != api.HealthcheckStatusOK {
		if err := os.MkdirAll(r.outputsDir, 0777); err == nil {
			msg := "outputs dir created successfully"
			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusOK, Message: msg}
			fixes = append(fixes, it)
		} else {
			msg := fmt.Sprintf("failed to create outputs dir: %s", err)
			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusFailed, Message: msg}
			fixes = append(fixes, it)
		}
	}

	return &api.HealthcheckReport{
		Checks: []api.HealthcheckItem{redisCheck, outputsDirCheck},
		Fixes:  fixes,
	}, nil
}

func (r *LocalExecutableRunner) Close() error {
	if r.redisCloseFn != nil {
		r.redisCloseFn()
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

	if err := pretty.Wait(); err != nil {
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

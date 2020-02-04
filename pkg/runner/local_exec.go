package runner

import (
	"context"
	"fmt"

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
	_ api.Runner = (*LocalExecutableRunner)(nil)
)

type LocalExecutableRunner struct {
	redisLk sync.Mutex
}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (r *LocalExecutableRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		plan        = input.TestPlan
		seq         = input.Seq
		name        = plan.Name
		redisWaitCh = make(chan struct{})
	)

	if seq >= len(plan.TestCases) {
		return nil, fmt.Errorf("invalid sequence number %d for test %s", seq, name)
	}

	// Housekeeping. If we've started a temporary redis instance for this test,
	// this defer will keep the runtime alive until it's shut down, giving us an
	// opportunity to print the "redis stopped successfully" log statement.
	// Otherwise, it might not be printed out at all.
	defer func() { <-redisWaitCh }()

	// Check if a local Redis instance is running. If not, try to start it.
	r.redisLk.Lock()
	if _, err := net.Dial("tcp", "localhost:6379"); err == nil {
		logging.S().Info("local redis instance check: OK")
		close(redisWaitCh)
	} else {
		// Try to start a Redis instance.
		logging.S().Info("local redis instance check: FAIL; attempting to start one for this run")

		// This context gets cancelled when the runner has finished, which in
		// turn signals the temporary Redis instance to shut down.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		cmd := exec.CommandContext(ctx, "redis-server", "--save", "\"\"", "--appendonly", "no")
		if err := cmd.Start(); err == nil {
			logging.S().Info("temporary redis instance started successfully")
		} else {
			close(redisWaitCh)
			r.redisLk.Unlock()
			return nil, fmt.Errorf("temporary redis instance failed to start: %w", err)
		}

		// This goroutine monitors the redis instance, and prints a log output
		// when it's done. The cmd.Wait() returns when the context is cancelled,
		// which happens when the runner finishes. Once we print the log
		// statement, we close the redis wait channel, which allows the method
		// to return.
		go func() {
			_ = cmd.Wait()
			logging.S().Info("temporary redis instance stopped successfully")
			close(redisWaitCh)
		}()
	}
	r.redisLk.Unlock()

	// Build a template runenv.
	template := runtime.RunEnv{
		TestPlan:          input.TestPlan.Name,
		TestCase:          input.TestPlan.TestCases[seq].Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       false,
		TestSubnet:        localSubnet,
	}

	// Spawn as many instances as the input parameters require.
	console := NewEventManager(NewConsoleLogger())
	commands := make([]*exec.Cmd, 0, input.TotalInstances)
	defer func() {
		for _, cmd := range commands {
			_ = cmd.Process.Kill()
		}
		for _, cmd := range commands {
			_ = cmd.Wait()
		}
		_ = console.Wait()
	}()

	var total int
	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupID = g.ID
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestInstanceParams = g.Parameters

		env := conv.ToOptionsSlice(runenv.ToEnvVars())

		for i := 0; i < g.Instances; i++ {
			total++
			logging.S().Infow("starting test case instance", "plan", name, "group", g.ID, "number", i, "total", total)

			id := fmt.Sprintf("instance %3d", total)
			cmd := exec.CommandContext(ctx, g.ArtifactPath)
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()
			cmd.Env = env

			if err := cmd.Start(); err != nil {
				console.FailStart(id, err)
				continue
			}

			commands = append(commands, cmd)

			console.Manage(id, stdout, stderr)
		}
	}

	if err := console.Wait(); err != nil {
		return nil, err
	}

	return &api.RunOutput{}, nil
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

func (*LocalExecutableRunner) CollectOutputs(runID string, w io.Writer) error {
	// TODO
	panic("unimplemented")
}

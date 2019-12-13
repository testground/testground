package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"reflect"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
)

var _ api.Runner = (*LocalExecutableRunner)(nil)

type LocalExecutableRunner struct{}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (*LocalExecutableRunner) Run(input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		plan        = input.TestPlan
		seq         = input.Seq
		instances   = input.Instances
		name        = plan.Name
		redisWaitCh = make(chan struct{})
	)

	// Housekeeping. If we've started a temporary redis instance for this test,
	// this defer will keep the runtime alive until it's shut down, giving us an
	// opportunity to print the "redis stopped successfully" log statement.
	// Otherwise, it might not be printed out at all.
	defer func() { <-redisWaitCh }()

	if seq >= len(plan.TestCases) {
		return nil, fmt.Errorf("invalid sequence number %d for test %s", seq, name)
	}

	// Check if a local Redis instance is running. If not, try to start it.
	if _, err := net.Dial("tcp", "localhost:6379"); err == nil {
		fmt.Fprintln(ow, "local redis instance check: OK")
		close(redisWaitCh)
	} else {
		// Try to start a Redis instance.
		fmt.Fprintln(ow, "local redis instance check: FAIL; attempting to start one for this run")

		// This context gets cancelled when the runner has finished, which in
		// turn signals the temporary Redis instance to shut down.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cmd := exec.CommandContext(ctx, "redis-server", "--notify-keyspace-events", "$szxK")
		if err := cmd.Start(); err == nil {
			fmt.Fprintln(ow, "temporary redis instance started successfully")
		} else {
			close(redisWaitCh)
			return nil, fmt.Errorf("temporary redis instance failed to start: %w", err)
		}

		// This goroutine monitors the redis instance, and prints a log output
		// when it's done. The cmd.Wait() returns when the context is cancelled,
		// which happens when the runner finishes. Once we print the log
		// statement, we close the redis wait channel, which allows the method
		// to return.
		go func() {
			_ = cmd.Wait()
			fmt.Fprintln(ow, "temporary redis instance stopped successfully")
			close(redisWaitCh)
		}()
	}

	testcase := plan.TestCases[seq]

	// Build a runenv.
	runenv := &runtime.RunEnv{
		TestPlan:           input.TestPlan.Name,
		TestCase:           testcase.Name,
		TestRun:            input.RunID,
		TestCaseSeq:        seq,
		TestInstanceCount:  input.Instances,
		TestInstanceParams: input.Parameters,
		TestSidecar:        false,
	}

	// Spawn as many instances as the input parameters require.
	console := NewConsoleOutput()
	commands := make([]*exec.Cmd, 0, instances)
	defer func() {
		for _, cmd := range commands {
			_ = cmd.Process.Kill()
		}
		for _, cmd := range commands {
			_ = cmd.Wait()
		}
		_ = console.Wait()
	}()

	var env []string
	for k, v := range runenv.ToEnvVars() {
		env = append(env, k+"="+v)
	}

	for i := 0; i < instances; i++ {
		logging.S().Infow("starting test case instance", "testcase", name, "runenv", env)
		id := fmt.Sprintf("instance %3d", i)

		cmd := exec.Command(input.ArtifactPath)
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
	if err := console.Wait(); err != nil {
		return nil, err
	}
	return new(api.RunOutput), nil
}

func (*LocalExecutableRunner) ID() string {
	return "local:exec"
}

func (*LocalExecutableRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(&LocalExecutableRunnerCfg{})
}

func (*LocalExecutableRunner) CompatibleBuilders() []string {
	return []string{"exec:go"}
}

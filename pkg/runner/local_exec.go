package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"reflect"
	"strconv"
	"sync"

	"github.com/logrusorgru/aurora"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
)

var _ api.Runner = (*LocalExecutableRunner)(nil)

type LocalExecutableRunner struct{}

// LocalExecutableRunnerCfg is the configuration struct for this runner.
type LocalExecutableRunnerCfg struct{}

func (*LocalExecutableRunner) Run(input *api.RunInput) (*api.RunOutput, error) {
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
		fmt.Println("local redis instance check: OK")
		close(redisWaitCh)
	} else {
		// Try to start a Redis instance.
		fmt.Println("local redis instance check: FAIL; attempting to start one for this run")

		// This context gets cancelled when the runner has finished, which in
		// turn signals the temporary Redis instance to shut down.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cmd := exec.CommandContext(ctx, "redis-server", "--notify-keyspace-events", "$szxK")
		if err := cmd.Start(); err == nil {
			fmt.Println("temporary redis instance started successfully")
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
			fmt.Println("temporary redis instance stopped successfully")
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
	var wg sync.WaitGroup
	for i := 0; i < instances; i++ {
		var env []string
		for k, v := range runenv.ToEnvVars() {
			env = append(env, k+"="+v)
		}

		logging.S().Infow("starting test case instance", "testcase", name, "runenv", env)

		a := aurora.NewAurora(logging.IsTerminal())

		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			var (
				nstr      = strconv.Itoa(n)
				color     = uint8(n%15) + 1
				cmd       = exec.Command(input.ArtifactPath)
				stdout, _ = cmd.StdoutPipe()
				stderr, _ = cmd.StderrPipe()
				combined  = io.MultiReader(stdout, stderr)
				scanner   = bufio.NewScanner(combined)
			)

			cmd.Env = env

			if err := cmd.Start(); err != nil {
				fmt.Println(a.Index(color, "<< instance "+nstr+" >>"), err)
				return
			}
			defer cmd.Wait() //nolint

			for scanner.Scan() {
				fmt.Println(a.Index(color, "<< instance "+nstr+" >>"), scanner.Text())
			}
		}(i)
	}

	wg.Wait()

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

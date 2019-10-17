package runner

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strconv"
	"sync"

	"github.com/logrusorgru/aurora"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
)

type LocalExecutableRunner struct{}

type LocalExecutableRunnerCfg struct{}

var _ api.Runner = (*LocalExecutableRunner)(nil)

func (*LocalExecutableRunner) Run(input *api.RunInput) (*api.RunOutput, error) {
	var (
		plan      = input.TestPlan
		seq       = input.Seq
		instances = input.Instances
		name      = plan.Name
	)

	if seq >= len(plan.TestCases) {
		return nil, fmt.Errorf("invalid sequence number %d for test %s", seq, name)
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
	}

	// Spawn as many instances as the test case dictates.
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
			cmd.Start()

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

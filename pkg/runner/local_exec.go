package runner

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strconv"
	"sync"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/google/uuid"
)

type LocalExecutableRunner struct{}

var _ Runner = (*LocalExecutableRunner)(nil)

func (*LocalExecutableRunner) Run(input *Input) (*Output, error) {
	var (
		plan      = input.TestPlan
		seq       = input.Seq
		instances = input.Instances
		name      = plan.Name
	)

	if seq >= len(plan.TestCases) {
		return nil, fmt.Errorf("invalid sequence number %d for test %s", seq, name)
	}

	tcase := plan.TestCases[seq]

	// Validate instance count.
	if instances == 0 {
		instances = tcase.Instances.Default
	} else if instances < tcase.Instances.Minimum || tcase.Instances.Maximum > instances {
		str := "instance count outside (%d) of allowable range [%d, %d] for test %s"
		err := fmt.Errorf(str, instances, tcase.Instances.Minimum, tcase.Instances.Maximum, name)
		return nil, err
	}

	// Spawn as many instances as the test case dictates.
	var wg sync.WaitGroup
	for i := 0; i < instances; i++ {
		cmd := exec.Command(input.ArtifactPath)

		// Populate the environment.
		env := map[string]string{
			runtime.EnvTestPlan:          name,
			runtime.EnvTestRun:           uuid.New().String(),
			runtime.EnvTestBranch:        "<TODO>",
			runtime.EnvTestTag:           "<TODO>",
			runtime.EnvTestCaseSeq:       strconv.Itoa(input.Seq),
			runtime.EnvTestInstanceCount: strconv.Itoa(instances),
		}

		cmd.Env = make([]string, 0, len(env))
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}

		logging.S().Infow("starting test case instance", "testcase", name, "runenv", cmd.Env)

		wg.Add(1)
		go func() {
			var (
				stdout, _ = cmd.StdoutPipe()
				stderr, _ = cmd.StderrPipe()
				combined  = io.MultiReader(stdout, stderr)
				scanner   = bufio.NewScanner(combined)
			)

			cmd.Start()

			for scanner.Scan() {
				l := scanner.Text()
				fmt.Println(l)
			}

			wg.Done()
		}()
	}
	wg.Wait()

	return new(Output), nil
}

func (*LocalExecutableRunner) ID() string {
	return "local:exec"
}

func (*LocalExecutableRunner) ConfigType() reflect.Type {
	// TODO
	return nil
}

func (*LocalExecutableRunner) CompatibleBuilders() []string {
	// TODO
	return nil
}

package main

import (
	"errors"
	"fmt"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.Invoke(runf)
}

// Pick a different example function to run
// depending on the name of the test case.
func runf(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "output":
		return ExampleOutput(runenv)
	case "failure":
		return ExampleFailure(runenv)
	case "panic":
		return ExamplePanic(runenv)
	case "params":
		return ExampleParams(runenv)
	case "sync":
		return ExampleSync(runenv)
	case "metrics":
		return ExampleMetrics(runenv)
	case "artifact":
		return ExampleArtifact(runenv)
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

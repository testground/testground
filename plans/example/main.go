package main

import (
	"errors"
	"fmt"

	"github.com/testground/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

// Pick a different example function to run
// depending on the name of the test case.
func run(runenv *runtime.RunEnv) error {
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
	case "prometheus":
		return ExamplePrometheus(runenv)
	case "prometheus2":
		return ExamplePrometheus2(runenv)
	case "prometheus3":
		return ExamplePrometheus3(runenv)
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

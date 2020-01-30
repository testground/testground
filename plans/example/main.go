package main

import (
	"errors"
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
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
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

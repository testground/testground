package main

import (
	"errors"
	"fmt"
	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

// Demonstrate test output functions
// This method emits two Messages and one Metric
func run(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "output":
		return ExampleOutput(runenv)
	case "failure":
		return ExampleFailure(runenv)
	case "params":
		return ExampleParams(runenv)
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

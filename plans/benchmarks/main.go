package main

import (
	"errors"
	"fmt"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.Invoke(tests)
}

// Pick a different example function to run
// depending on the name of the test case.
func tests(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "startup":
		return StartTimeBench(runenv)
	case "netinit":
		return NetworkInitBench(runenv)
	case "netlinkshape":
		return NetworkLinkShapeBench(runenv)
	case "barrier":
		return BarrierBench(runenv)
	case "subtree":
		return SubtreeBench(runenv)
	case "storm":
		return Storm(runenv)
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

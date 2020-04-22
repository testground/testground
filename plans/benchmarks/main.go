package main

import (
	"errors"
	"fmt"

	"github.com/testground/sdk-go/runtime"
)

func main() {
	runtime.Invoke(run)
}

// Pick a different example function to run
// depending on the name of the test case.
func run(runenv *runtime.RunEnv) error {
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
	case "all":
		_ = StartTimeBench(runenv)
		_ = NetworkInitBench(runenv)
		_ = NetworkLinkShapeBench(runenv)
		_ = BarrierBench(runenv)
		_ = SubtreeBench(runenv)
		return nil
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

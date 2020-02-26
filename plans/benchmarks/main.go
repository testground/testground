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
	case "startup":
		return StartTimeBench(runenv)
	case "netinit":
		return NetworkInitBench(runenv)
	case "barrier":
		return BarrierBench(runenv)
	case "all":
		_ = StartTimeBench(runenv)
		_ = NetworkInitBench(runenv)
		_ = BarrierBench(runenv)
		return nil
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

package main

import (
	"errors"
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "signalsizebench":
		return signalSizeBench(runenv)
	case "signalconcurrencybench":
		return signalConcurrencyBench(runenv)
	case "barrierbench":
		return signalBarrierBench(runenv)
	default:
		msg := fmt.Sprintf("Unknown Testcase %s", c)
		return errors.New(msg)
	}
}

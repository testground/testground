package main

import (
	test "github.com/ipfs/testground/plans/bitswap-tuning/test"
	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "transfer":
		return test.Transfer(runenv)
	case "fuzz":
		return test.Fuzz(runenv)
	default:
		panic("unrecognized test case")
	}
}

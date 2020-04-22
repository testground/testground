package main

import (
	test "github.com/testground/testground/plans/bitswap-tuning/test"
	"github.com/testground/sdk-go/runtime"
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

package main

import (
	test "github.com/ipfs/testground/plans/autonat/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []func(*runtime.RunEnv) error{
	test.PublicNodes,
}

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}
	return testCases[runenv.TestCaseSeq](runenv)
}

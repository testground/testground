package main

import (
	"github.com/ipfs/testground/plans/secure-channel/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []func(*runtime.RunEnv) error {
	test.TestDataTransfer,
}

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	// Demux to the right test case.
	return testCases[runenv.TestCaseSeq](runenv)
}

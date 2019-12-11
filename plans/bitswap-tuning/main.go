package main

import (
	test "github.com/ipfs/testground/plans/bitswap-tuning/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []func(*runtime.RunEnv){
	test.Transfer,
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	// Demux to the right test case.
	testCases[runenv.TestCaseSeq](runenv)
}

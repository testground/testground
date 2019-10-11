package main

import (
  "github.com/ipfs/testground/sdk/runtime"
  test "github.com/ipfs/testground/plans/dht/test"
)

var testCases = []func(*runtime.RunEnv){
	test.LookupPeers,
	test.LookupProviders,
	test.StoreGetValue,
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	// Demux to the right test case.
	testCases[runenv.TestCaseSeq](runenv)
}

package main

import "github.com/ipfs/testground/sdk/runtime"

var testCases = []func(*runtime.RunEnv){
	LookupPeers,
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	// Demux to the right test case.
	testCases[runenv.TestCaseSeq](runenv)
}

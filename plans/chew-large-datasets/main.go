package main

import (
	"github.com/ipfs/testground/sdk/runtime"
	test "github.com/ipfs/testground/plans/chew-large-datasets/test"
)

var testCases = []func(*runtime.RunEnv){
	test.IpfsAddDefaults,
	test.IpfsAddDirSharding,
	test.IpfsAddTrickleDag,
	test.IpfsMfs,
	test.IpfsMfsDirSharding,
	test.IpfsUrlStore,
	test.IpfsFileStore,
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	// Demux to the right test case.
	testCases[runenv.TestCaseSeq](runenv)
}

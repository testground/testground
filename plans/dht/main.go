package main

import (
	test "github.com/ipfs/testground/plans/dht/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = map[string]func(*runtime.RunEnv) error{
	"find-peers": test.FindPeers,
	"find-providers" : test.FindProviders,
	"provide-stress" : test.ProvideStress,
	"store-get-value" : test.StoreGetValue,
	"barrier" : test.Barrier,
}

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}
	return testCases[runenv.TestCase](runenv)
}

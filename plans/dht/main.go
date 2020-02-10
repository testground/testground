package main

import (
	"github.com/ipfs/testground/plans/dht/metrics"
	test "github.com/ipfs/testground/plans/dht/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []func(*runtime.RunEnv) error{
	test.FindPeers,
	test.FindProviders,
	test.ProvideStress,
	test.StoreGetValue,
}

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}
	metrics.Setup()
	defer metrics.EmitMetrics()
	return testCases[runenv.TestCaseSeq](runenv)
}

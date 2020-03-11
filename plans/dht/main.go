package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	test "github.com/ipfs/testground/plans/dht/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []func(*runtime.RunEnv) error{
	test.FindPeers,
	test.FindProviders,
	test.ProvideStress,
	test.StoreGetValue,
	test.GetClosestPeers,
	test.BarrierTest,
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}
	return testCases[runenv.TestCaseSeq](runenv)
}

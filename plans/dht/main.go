package main

import (
	test "github.com/ipfs/testground/plans/dht/test"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = map[string]runtime.TestCaseFn{
	"find-peers": test.FindPeers,
	"find-providers": test.FindProviders,
	"provide-stress": test.ProvideStress,
	"store-get-value": test.StoreGetValue,
	"get-closest-peers": test.GetClosestPeers,
	"bootstrap-network": test.BootstrapNetwork,
	"all": test.All,
}

func main() {
	runtime.InvokeMap(testCases)
}

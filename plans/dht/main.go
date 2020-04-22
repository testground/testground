package main

import (
	test "github.com/testground/testground/plans/dht/test"
	"github.com/testground/sdk-go/runtime"
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

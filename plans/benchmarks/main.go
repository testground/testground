package main

import (
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(testcases)
}

var testcases = map[string]interface{}{
	"startup":      run.InitializedTestCaseFn(StartTimeBench),
	"netinit":      run.InitializedTestCaseFn(NetworkInitBench),
	"netlinkshape": run.InitializedTestCaseFn(NetworkLinkShapeBench),
	"barrier":      run.InitializedTestCaseFn(BarrierBench),
	"subtree":      run.InitializedTestCaseFn(SubtreeBench),
	"storm":        run.InitializedTestCaseFn(Storm),
}

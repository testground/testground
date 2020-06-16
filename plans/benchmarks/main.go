package main

import "github.com/testground/sdk-go/run"

func main() {
	run.InvokeMap(map[string]interface{}{
		"startup":      StartTimeBench,
		"netinit":      NetworkInitBench,
		"netlinkshape": NetworkLinkShapeBench,
		"barrier":      BarrierBench,
		"subtree":      SubtreeBench,
		"storm":        Storm,
	})
}

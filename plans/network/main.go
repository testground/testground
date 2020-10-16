package main

import (
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
)

var testcases = map[string]interface{}{
	"ping-pong":       run.InitializedTestCaseFn(pingpong),
	"traffic-allowed": routingPolicyTest(network.AllowAll),
	"traffic-blocked": routingPolicyTest(network.DenyAll),
}

func main() {
	run.InvokeMap(testcases)
}

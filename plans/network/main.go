package main

import (
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
)

var testcases = map[string]interface{}{
	"ping-pong":       pingpong,
	"traffic-allowed": makeTest(network.AllowAll),
	"traffic-blocked": makeTest(network.DenyAll),
}

func main() {
	run.InvokeMap(testcases)
}

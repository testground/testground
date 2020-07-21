package main

import (
	"github.com/testground/sdk-go/run"
)

var testcases = map[string]interface{}{
	"ping-pong":       pingpong,
	"traffic-allowed": makeTest(true),
	"traffic-blocked": makeTest(false),
}

func main() {
	run.InvokeMap(testcases)
}

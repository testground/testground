package main

import (
	"os"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// Demonstrate test output functions
// This method emits two Messages and one Metric
func ExampleOutput(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	if msg := os.Getenv("TESTGROUND_HELLO_MESSAGE"); msg != "" {
		runenv.RecordMessage(msg)
	} else {
		runenv.RecordMessage("Hello, World.")
	}
	runenv.RecordMessage("Additional arguments: %d", len(runenv.TestInstanceParams))
	runenv.R().RecordPoint("donkeypower", 3.0)
	return nil
}

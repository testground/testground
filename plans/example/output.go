package main

import (
	"github.com/testground/sdk-go/runtime"
)

// Demonstrate test output functions
// This method emits two Messages and one Metric
func ExampleOutput(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Hello, World.")
	runenv.RecordMessage("Additional arguments: %d", len(runenv.TestInstanceParams))
	runenv.R().RecordPoint("donkeypower", 3.0)
	return nil
}

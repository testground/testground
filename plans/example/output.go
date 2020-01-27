package main

import (
	"github.com/ipfs/testground/sdk/runtime"
)

// Demonstrate test output functions
// This method emits two Messages and one Metric
func ExampleOutput(runenv *runtime.RunEnv) error {
	runenv.Message("Hello, World.")
	runenv.Message("Additional arguments ", runenv.TestInstanceParams)
	def := runtime.MetricDefinition{
		Name:           "donkeypower",
		Unit:           "kiloforce",
		ImprovementDir: -1,
	}
	runenv.EmitMetric(&def, 3.0)
	return nil
}

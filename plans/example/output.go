package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// Demonstrate test output functions
// This method emits two Messages and one Metric
func ExampleOutput(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("Hello, World.")
	runenv.RecordMessage("Additional arguments: %d", len(runenv.TestInstanceParams))
	runenv.R().RecordPoint("donkeypower", 3.0)

	// Demo structured assets:
	// After you `testground run` this, you may use `testground collect <runid>` to
	// get a tar which contains the test outputs for every instances, including a `demo.out` file.
	_, l, err := runenv.CreateStructuredAsset("demo.out", runtime.StandardJSONConfig())

	if (err != nil) {
		return err
	}

	l.Info("This is a structured output message")

	return nil
}

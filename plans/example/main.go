package main

import "github.com/testground/sdk-go/run"

func main() {
	run.InvokeMap(map[string]interface{}{
		"output":   ExampleOutput,
		"failure":  ExampleFailure,
		"panic":    ExamplePanic,
		"params":   ExampleParams,
		"sync":     ExampleSync,
		"metrics":  ExampleMetrics,
		"artifact": ExampleArtifact,
	})
}

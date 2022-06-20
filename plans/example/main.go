package main

import (
	"github.com/testground/sdk-go/run"
)

func main() {
	run.InvokeMap(testcases)
}

var testcases = map[string]interface{}{
	"output":   run.InitializedTestCaseFn(ExampleOutput),
	"failure":  run.InitializedTestCaseFn(ExampleFailure),
	"panic":    run.InitializedTestCaseFn(ExamplePanic),
	"params":   run.InitializedTestCaseFn(ExampleParams),
	"sync":     run.InitializedTestCaseFn(ExampleSync),
	"metrics":  run.InitializedTestCaseFn(ExampleMetrics),
	"artifact": run.InitializedTestCaseFn(ExampleArtifact),
}

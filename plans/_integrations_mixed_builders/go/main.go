package main

import (
	"errors"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-1357-mix-builder-configuration": run.InitializedTestCaseFn(overrideBuilderConfiguration),
}

func main() {
	run.InvokeMap(testcases)
}

func overrideBuilderConfiguration(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	implementation := "go"
	expectedImplementation := runenv.StringParam("expected_implementation")
	runenv.RecordMessage("Builder Configuration run with implementation: %s, expected implementation: %s", implementation, expectedImplementation)

	if expectedImplementation != implementation {
		return errors.New("expected version does not match")
	}

	return nil
}

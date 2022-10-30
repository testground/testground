package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-tbd-workspaces-support": run.InitializedTestCaseFn(tbd),
}

func main() {
	run.InvokeMap(testcases)
}

func tbd(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordSuccess()
	return nil
}

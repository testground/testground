package main

import (
	"errors"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.InvokeMap(testcases)
}

var testcases = map[string]interface{}{
	"ok":    run.InitializedTestCaseFn(ok),
	"panic": run.InitializedTestCaseFn(panickingTest),
	"stall": run.InitializedTestCaseFn(stall),
}

func ok(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	return nil
}

func panickingTest(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	panic(errors.New("this is an intentional panic"))
}

func stall(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("Now stalling for 24 hours")
	time.Sleep(24 * time.Hour)
	return nil
}

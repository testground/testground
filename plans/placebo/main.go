package main

import (
	"errors"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"ok":    run.InitializedTestCaseFn(tcOk),
	"panic": run.InitializedTestCaseFn(tcPanic),
	"abort": run.InitializedTestCaseFn(tcAbort),
	"stall": run.InitializedTestCaseFn(tcStall),
}

func main() {
	run.InvokeMap(testcases)
}

func tcOk(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	return nil
}

func tcAbort(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	return errors.New("aborting...")
}

func tcStall(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	time.Sleep(24 * time.Hour)
	return nil
}

func tcPanic(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	panic(errors.New("this is an intentional panic"))
}

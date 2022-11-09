package main

import (
	"errors"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-1349-silent-failure": silentFailure,
	"issue-1493-success": run.InitializedTestCaseFn(success),
	"issue-1493-failure": run.InitializedTestCaseFn(failure),
}

func main() {
	run.InvokeMap(testcases)
}

func silentFailure(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("This fails by NOT returning an error and NOT sending a test success status.")
	return nil
}

func success(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("success!")
	return nil
}

func failure(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("failure!")
	return errors.New("expected failure")
}

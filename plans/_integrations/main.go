package main

import (
	"errors"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-1170-simple-success": run.InitializedTestCaseFn(success),
	"issue-1349-silent-failure": silentFailure,
	"issue-1493-success": run.InitializedTestCaseFn(success),
	"issue-1493-optional-failure": run.InitializedTestCaseFn(optionalFailure),
	"issue-1542-stalled-test-panic": run.InitializedTestCaseFn(panickingTest),
	"issue-1542-stalled-test-stall": run.InitializedTestCaseFn(stallingTest),
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

func optionalFailure(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	shouldFail := runenv.BooleanParam("should_fail")
	runenv.RecordMessage("Test run with shouldFail: %s", shouldFail)

	if shouldFail  {
		return errors.New("failing as requested")
	}

	return nil
}

func panickingTest(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Only the first instance panics
	if initCtx.GlobalSeq == 1 {
		runenv.RecordMessage("panicking on purpose")
		panic(errors.New("this is an intentional panic"))
	}

	runenv.RecordMessage("container completed successfully")
	return nil
}

func stallingTest(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	// Only the first instance stalls
	if initCtx.GlobalSeq == 1 {
		runenv.RecordMessage("stalling on purpose")
		time.Sleep(24 * time.Hour)
	}

	runenv.RecordMessage("container completed successfully")
	return nil
}
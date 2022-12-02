package main

import (
	"errors"

	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-1349-silent-failure":   silentFailure,
	"issue-1432-task-timeout":     taskTimeout,
	"issue-1493-success":          run.InitializedTestCaseFn(success),
	"issue-1493-optional-failure": run.InitializedTestCaseFn(optionalFailure),
}

func main() {
	run.InvokeMap(testcases)
}

func silentFailure(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("This fails by NOT returning an error and NOT sending a test success status.")
	return nil
}

func taskTimeout(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Sleeping...")
	time.Sleep(2 * time.Minute)
	runenv.RecordMessage("Hello I woke up...")
	return nil
}

func success(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("success!")
	return nil
}

func optionalFailure(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	shouldFail := runenv.BooleanParam("should_fail")
	runenv.RecordMessage("Test run with shouldFail: %s", shouldFail)

	if shouldFail {
		return errors.New("failing as requested")
	}

	return nil
}

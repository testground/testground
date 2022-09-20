package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"time"
)

var testcases = map[string]interface{}{
	"issue-1349-silent-failure": silentFailure,
	"issue-1432-task-timeout":   taskTimeout,
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

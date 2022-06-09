package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

var testcases = map[string]interface{}{
	"issue-1349-silent-failure": silentFailure,
}

func main() {
	run.InvokeMap(testcases)
}

func silentFailure(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("This fails by NOT returning an error and NOT sending a test success status.")
	return nil
}

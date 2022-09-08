package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"time"
)

var testcases = map[string]interface{}{
	"sleep": sleep,
}

func main() {
	run.InvokeMap(testcases)
}

func sleep(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Sleeping...")
	time.Sleep(70 * time.Second)
	runenv.RecordMessage("Hello I woke up...")
	return nil
}

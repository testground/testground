package main

import (
	"context"
	"errors"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

func main() {
	run.InvokeMap(testcases)
}

var testcases = map[string]interface{}{
	"ok":    ok,
	"panic": panickingTest,
	"stall": stall,
}

func ok(runenv *runtime.RunEnv) error {
	minimalInit(runenv)

	return nil
}

func panickingTest(runenv *runtime.RunEnv) error {
	minimalInit(runenv)

	panic(errors.New("this is an intentional panic"))
}

func stall(runenv *runtime.RunEnv) error {
	minimalInit(runenv)

	runenv.RecordMessage("Now stalling for 24 hours")
	time.Sleep(24 * time.Hour)
	return nil
}

// Initialize the sync client and attach it,
// This replaces the `run.InitializedTestCaseFn` wrapper for
// tests that are used in integration, where the `MustWaitNetworkInitialized` call hangs.
//
// Do not close the client because the SDK uses this at the end of the test to send the success event
// to the sync service.
func minimalInit(runenv *runtime.RunEnv) {
	ctx := context.Background()
	client := sync.MustBoundClient(ctx, runenv)
	runenv.AttachSyncClient(client)
}

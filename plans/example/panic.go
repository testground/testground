package main

import (
	"github.com/ipfs/testground/sdk/runtime"
)

// ExamplePanic always panics
// This method shows what happens when the test plans fails without returning an error
func ExamplePanic(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("About to hit an unhandled error")
	panic("intentional panic")
}

package main

import (
	"errors"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// ExampleFailure always fails
func ExampleFailure(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("This is what happens when there is a failure")
	return errors.New("intentional oops")
}

package main

import (
	"errors"

	"github.com/testground/sdk-go/runtime"
)

// ExampleFailure always fails
func ExampleFailure(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("This is what happens when there is a failure")
	return errors.New("intentional oops")
}

package main

import (
	"errors"
	"github.com/ipfs/testground/sdk/runtime"
)

// ExampleFailure always fails
func ExampleFailure(runenv *runtime.RunEnv) error {
	runenv.Message("This is what happens when there is a failure")
	return errors.New("intentional oops")
}

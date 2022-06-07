package main

import (
	"io/ioutil"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// This only works when docker:generic builder is used.
func ExampleArtifact(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	a, err := ioutil.ReadFile("/artifact.txt")
	if err != nil {
		runenv.RecordFailure(err)
		return err
	}
	runenv.RecordMessage(string(a))
	return nil
}

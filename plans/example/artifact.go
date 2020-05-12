package main

import (
	"github.com/testground/sdk-go/runtime"
	"io/ioutil"
)

// This only works when docker:generic builder is used.
func ExampleArtifact(runenv *runtime.RunEnv) error {
	a, err := ioutil.ReadFile("/artifact.txt")
	if err != nil {
		runenv.RecordFailure(err)
		return err
	}
	runenv.RecordMessage(string(a))
	return nil
}

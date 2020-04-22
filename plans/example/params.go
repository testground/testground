package main

import (
	"github.com/testground/testground/sdk/runtime"
)

// ExampleParams prints out the params passed to it.
func ExampleParams(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Params are defined in toml manifest")
	runenv.RecordMessage("Params can be overridden by the commandline!")
	for k, v := range runenv.TestInstanceParams {
		runenv.RecordMessage("key: %s, value: %s", string(k), string(v))
	}
	runenv.RecordMessage("The value of param2 is %s", runenv.StringParam("param2"))
	return nil
}

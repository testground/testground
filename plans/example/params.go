package main

import (
	"github.com/ipfs/testground/sdk/runtime"
)

// ExampleParams prints out the params passed to it.
func ExampleParams(runenv *runtime.RunEnv) error {
	runenv.Message("Params are defined in toml manifest")
	runenv.Message("Params can be overridden by the commandline!")
	for k, v := range runenv.TestInstanceParams {
		runenv.Message("key: %s, value: %s", string(k), string(v))
	}
	runenv.Message("The value of param2 is %s", runenv.StringParam("param2"))
	return nil
}

package main

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.Invoke(func(runenv *runtime.RunEnv) error {
		runenv.RecordMessage("hi there!")
		return nil
	})
}

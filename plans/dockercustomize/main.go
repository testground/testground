package main

import "github.com/testground/sdk-go/runtime"

func main() {
	runtime.Invoke(func(runenv *runtime.RunEnv) error {
		runenv.RecordMessage("hi there!")
		return nil
	})
}

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"additional_envs": run.InitializedTestCaseFn(additionalEnvs),
	})
}

func additionalEnvs(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	msg, found := os.LookupEnv("TESTGROUND_HELLO_MESSAGE")
	if !found {
		return errors.New("No env set with key: TESTGROUND_HELLO_MESSAGE")
	}
	if msg != "Hello, AdditionalEnvs!" {
		return fmt.Errorf("unexpected TESTGROUND_HELLO_MESSAGE: %q", msg)
	}
	return nil
}

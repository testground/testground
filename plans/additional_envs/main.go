package main

import (
	"fmt"
	"os"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

const ENV_KEY = "MY_FIRST_CUSTOM_ENV_VARIABLE"

func main() {
	run.InvokeMap(map[string]interface{}{
		"additional_envs": run.InitializedTestCaseFn(additionalEnvs),
	})
}

func additionalEnvs(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	msg, found := os.LookupEnv(ENV_KEY)
	if !found {
		return fmt.Errorf("No env set with key: %s", ENV_KEY)
	}
	if msg != "Hello, AdditionalEnvs!" {
		return fmt.Errorf("unexpected %s: %q", ENV_KEY, msg)
	}
	return nil
}

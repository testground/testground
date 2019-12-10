package test

import (
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

func Ping(runenv *runtime.RunEnv) {
	var (
		timeout = time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second
	)

	deadline := time.After(timeout)
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-deadline:
			runenv.OK()
			return

		case <-tick:
			fmt.Println("tick...")
		}
	}
}

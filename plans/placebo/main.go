package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.Invoke(runf)
}

func runf(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "ok":
		return nil
	case "panic":
		// panic
		panic(errors.New("this is an intentional panic"))
	case "stall":
		// stall
		time.Sleep(24 * time.Hour)
		return nil
	default:
		return fmt.Errorf("aborting")
	}
}

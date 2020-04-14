package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "ok":
		return nil
	case "metrics":
		// create context for cancelation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// snapshot metrics every second and save them into "metrics" directory
		err := runenv.HTTPPeriodicSnapshots(ctx, "http://"+runtime.HTTPListenAddr+"/metrics", time.Second, "metrics")
		if err != nil {
			return err
		}

		time.Sleep(time.Second * 5)
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

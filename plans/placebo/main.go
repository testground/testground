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
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	switch runenv.TestCaseSeq {
	case 0:
		return nil
	case 2:
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
	case 3:
		// panic
		panic(errors.New("this is an intentional panic"))
	case 4:
		// stall
		time.Sleep(24 * time.Hour)
		return nil
	default:
		return fmt.Errorf("aborting")
	}
}

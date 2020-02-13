package main

import (
	"context"
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
		// expose prometheus endpoint
		listener := runenv.MustExportPrometheus()
		defer listener.Close()

		// create context for cancelation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// snapshot metrics every second and save them into "metrics" directory
		err := runenv.HTTPPeriodicSnapshots(ctx, "http://"+listener.Addr().String(), time.Second, "metrics")
		if err != nil {
			return err
		}

		time.Sleep(time.Minute)
		return nil
	default:
		return fmt.Errorf("aborting")
	}
}

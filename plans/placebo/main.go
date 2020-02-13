package main

import (
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
		addr := runenv.MustExportPrometheus()
		go runenv.HTTPPeriodicSnapshots(addr, time.Second, "metrics-$TIME.out")
		time.Sleep(time.Second * 15)
		return nil
	default:
		return fmt.Errorf("aborting")
	}
}

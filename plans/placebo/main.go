package main

import (
	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	if runenv.TestCaseSeq == 0 {
		runenv.OK()
	} else {
		runenv.Abort("aborting")
	}
}

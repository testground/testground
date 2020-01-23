package main

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}
func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	if runenv.TestCaseSeq != 0 {
		return fmt.Errorf("aborting")
	}
	return nil
}

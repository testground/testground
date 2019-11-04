package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddDirSharding(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsAddDirSharing")

	runenv.OK()
}

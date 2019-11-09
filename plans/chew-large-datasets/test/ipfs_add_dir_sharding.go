package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddDirSharding(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsAddDirSharing")

	// Need https://github.com/ipfs/interface-go-ipfs-core/issues/48

	runenv.OK()
}

package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsFileStore(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsFileStore")

	runenv.OK()
}

package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsUrlStore(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsUrlStore")

	runenv.OK()
}

package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddDefaults(runenv *runtime.RunEnv) {
	fmt.Printf("Yo")

	runenv.OK()
}

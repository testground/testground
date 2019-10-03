package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddTrickleDag(runenv *runtime.RunEnv) {
	fmt.Printf("Yo")

	runenv.OK()
}

package test

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsMfs(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsMfs")

	runenv.OK()
}

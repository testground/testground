package test

import (
	"context"
	"fmt"
	"os"

	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDefaults IPFS Add Defaults Test
func IpfsAddDefaults(runenv *runtime.RunEnv) {
	var size int64 = 1024 * 1024

	ctx, _ := context.WithCancel(context.Background())
	ipfs, err := utils.CreateIpfsInstance(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	file, err := utils.CreateRandomFile(runenv, os.TempDir(), size)
	if err != nil {
		runenv.Abort(err)
		return
	}
	defer os.Remove(file.Name())

	unixfsFile, err := utils.GetPathToUnixfsFile(file.Name())
	if err != nil {
		runenv.Abort(fmt.Errorf("failed to get Unixfs file from path: %s", err))
		return
	}

	cidFile, err := ipfs.Unixfs().Add(ctx, unixfsFile)
	if err != nil {
		runenv.Abort(fmt.Errorf("Could not add File: %s", err))
		return
	}

	fmt.Printf("Added file to IPFS with CID %s\n", cidFile.String())

	runenv.OK()
}

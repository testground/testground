package test

import (
	"context"
	"fmt"
	"os"

	coreopts "github.com/ipfs/interface-go-ipfs-core/options"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddTrickleDag(runenv *runtime.RunEnv) {
	// TODO make this file size customizable by parameter
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

	addOptions := func(settings *coreopts.UnixfsAddSettings) error {
		settings.Layout = coreopts.TrickleLayout
		return nil
	}

	cidFile, err := ipfs.Unixfs().Add(ctx, unixfsFile, addOptions)
	if err != nil {
		runenv.Abort(fmt.Errorf("Could not add File: %s", err))
		return
	}

	fmt.Printf("Added file to IPFS with CID %s\n", cidFile.String())

	runenv.OK()
}

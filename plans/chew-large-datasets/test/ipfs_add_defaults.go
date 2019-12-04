package test

import (
	"context"
	"fmt"

	files "github.com/ipfs/go-ipfs-files"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDefaults IPFS Add Defaults Test
func IpfsAddDefaults(runenv *runtime.RunEnv) {
	cfg, err := utils.GetAddTestsConfig(runenv)
	if err != nil {
		runenv.Abort(err)
		return
	}

	ctx, _ := context.WithCancel(context.Background())
	ipfs, err := utils.CreateIpfsInstance(ctx, nil)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	err = cfg.ForEachSize(runenv, func (unixfsFile files.File) error {
		cidFile, err := ipfs.Unixfs().Add(ctx, unixfsFile)
		if err != nil {
			return fmt.Errorf("Could not add File: %s", err)
		}

		fmt.Printf("Added file to IPFS with CID %s\n", cidFile.String())
		return nil
	})

	if err != nil {
		runenv.Abort(err)
		return
	}

	runenv.OK()
}

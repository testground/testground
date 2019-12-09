package test

import (
	"context"
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	uio "github.com/ipfs/go-unixfs/io"
	coreopts "github.com/ipfs/interface-go-ipfs-core/options"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDirSharding IPFS Add Directory Sharding Test
func IpfsAddDirSharding(runenv *runtime.RunEnv) {
	ctx, _ := context.WithCancel(context.Background())
	ipfs, err := utils.CreateIpfsInstance(ctx, func(cfg *config.Config) error {
		cfg.Experimental.ShardingEnabled = true
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	if !uio.UseHAMTSharding {
		runenv.Abort(fmt.Errorf("failed to enable sharding"))
		return
	}

	err = utils.ForEachCase(runenv, func(unixfsFile files.Node, isDir bool) error {
		t := "file"
		if isDir {
			t = "directory"
		}

		addOptions := func(settings *coreopts.UnixfsAddSettings) error {
			settings.Layout = coreopts.TrickleLayout
			return nil
		}

		cidFile, err := ipfs.Unixfs().Add(ctx, unixfsFile, addOptions)
		if err != nil {
			return fmt.Errorf("Could not add %s: %s", t, err)
		}

		fmt.Printf("Added %s to IPFS with CID %s\n", t, cidFile.String())
		return nil
	})

	if err != nil {
		runenv.Abort(err)
		return
	}

	runenv.OK()
}

package test

import (
	"context"
	"fmt"

	files "github.com/ipfs/go-ipfs-files"
	coreopts "github.com/ipfs/interface-go-ipfs-core/options"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsFileStore IPFS File Store Test
func IpfsFileStore(runenv *runtime.RunEnv) {
	ctx, _ := context.WithCancel(context.Background())
	ipfs, err := utils.CreateIpfsInstance(ctx, nil)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	err = utils.ForEachCase(runenv, func(unixfsFile files.Node, isDir bool) error {
		t := "file"
		if isDir {
			t = "directory"
		}

		addOptions := func(settings *coreopts.UnixfsAddSettings) error {
			settings.NoCopy = true
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

	// TODO: Act II and Act III

	runenv.OK()
}

package test

import (
	"context"
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsAddDirSharding(runenv *runtime.RunEnv) {
	fmt.Printf("Yo - IpfsAddDirSharing")

	ctx, _ := context.WithCancel(context.Background())
	_, err := utils.CreateIpfsInstance(ctx, func (cfg *config.Config) error {
		cfg.Experimental.ShardingEnabled = true
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	fmt.Printf("%v", uio.UseHAMTSharding)

	runenv.OK()
}

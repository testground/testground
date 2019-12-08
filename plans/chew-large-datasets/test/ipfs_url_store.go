package test

import (
	"context"
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/runtime"
)

func IpfsUrlStore(runenv *runtime.RunEnv) {

	ctx, _ := context.WithCancel(context.Background())
	_, err := utils.CreateIpfsInstance(ctx, func(cfg *config.Config) error {
		cfg.Experimental.UrlstoreEnabled = true
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	// TODO: ...
    fmt.Printf("Test incomplete")

	runenv.OK()
}

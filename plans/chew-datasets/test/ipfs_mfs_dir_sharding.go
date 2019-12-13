package test

import (
	"context"

	utils "github.com/ipfs/testground/plans/chew-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsMfsDirSharding IPFS MFS Dir Sharding test
type IpfsMfsDirSharding struct{}

func (t *IpfsMfsDirSharding) AcceptFiles() bool {
	return false
}

func (t *IpfsMfsDirSharding) AcceptDirs() bool {
	return true
}

func (t *IpfsMfsDirSharding) AddRepoOptions() iptb.AddRepoOptions {
	return nil
}

func (t *IpfsMfsDirSharding) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
	if cfg.IpfsInstance != nil {
		runenv.Message("Running against the Core API")
		runenv.Message("Not Implemented Yet")
	}

	if cfg.IpfsDaemon != nil {
		runenv.Message("Running against the Daemon (IPTB)")
		runenv.Message("Not Implemented Yet")
	}

	runenv.OK()
}

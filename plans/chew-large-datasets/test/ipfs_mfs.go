package test

import (
	"context"
	"fmt"

	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsMfs IPFS MFS Add Test
type IpfsMfs struct{}

func (t *IpfsMfs) AcceptFiles() bool {
	return true
}

func (t *IpfsMfs) AcceptDirs() bool {
	return false
}

func (t *IpfsMfs) AddRepoOptions() iptb.AddRepoOptions {
	return nil
}

func (t *IpfsMfs) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
	if cfg.IpfsInstance != nil {
		fmt.Println("Running against the Core API")
		fmt.Println("Not Implemented Yet")
	}

	if cfg.IpfsDaemon != nil {
		fmt.Println("Running against the Daemon (IPTB)")
		fmt.Println("Not Implemented Yet")
	}

	runenv.OK()
}

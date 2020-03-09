package test

import (
	"context"
	"fmt"

	utils "github.com/ipfs/testground/plans/chew-datasets/utils"
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

func (t *IpfsMfs) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) error {
	if cfg.IpfsInstance != nil {
		runenv.RecordMessage("Running against the Core API")
		runenv.RecordMessage("Not Implemented Yet")
	}

	if cfg.IpfsDaemon != nil {
		runenv.RecordMessage("Running against the Daemon (IPTB)")
		runenv.RecordMessage("Not Implemented Yet")
	}

	return fmt.Errorf("not implemented")
}

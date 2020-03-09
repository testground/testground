package test

import (
	"context"
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
	utils "github.com/ipfs/testground/plans/chew-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsUrlStore IPFS MFS Url Store test
type IpfsUrlStore struct{}

func (t *IpfsUrlStore) AcceptFiles() bool {
	return true
}

func (t *IpfsUrlStore) AcceptDirs() bool {
	return true
}

func (t *IpfsUrlStore) AddRepoOptions() iptb.AddRepoOptions {
	return func(cfg *config.Config) error {
		cfg.Experimental.UrlstoreEnabled = true
		return nil
	}
}

func (t *IpfsUrlStore) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) error {
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

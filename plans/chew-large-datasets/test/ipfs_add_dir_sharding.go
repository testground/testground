package test

import (
	"context"
	"fmt"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDirSharding IPFS Add Directory Sharding Test
type IpfsAddDirSharding struct{}

func (t *IpfsAddDirSharding) AcceptFiles() bool {
	return false
}

func (t *IpfsAddDirSharding) AcceptDirs() bool {
	return true
}

func (t *IpfsAddDirSharding) InstanceOptions() *utils.IpfsInstanceOptions {
	return &utils.IpfsInstanceOptions{
		RepoOpts: func(cfg *config.Config) error {
			cfg.Experimental.ShardingEnabled = true
			return nil
		},
	}
}

func (t *IpfsAddDirSharding) DaemonOptions() *iptb.TestEnsembleSpec {
	return nil
}

func (t *IpfsAddDirSharding) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
	if cfg.IpfsInstance != nil {
		fmt.Println("Running against the Core API")

		err := cfg.Config.ForEachUnixfs(runenv, func(unixfsFile files.Node, isDir bool) (string, error) {
			cidFile, err := cfg.IpfsInstance.Unixfs().Add(ctx, unixfsFile)
			if err != nil {
				return "", err
			}

			return cidFile.String(), nil
		})

		if err != nil {
			runenv.Abort(err)
			return
		}
	}

	if cfg.IpfsDaemon != nil {
		fmt.Println("NOT IMPLEMENTED against the Daemon (IPTB)")
	}

	runenv.OK()
}

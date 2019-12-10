package test

import (
	"context"
	"fmt"
	"os"

	files "github.com/ipfs/go-ipfs-files"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddDefaults IPFS Add Defaults Test
type IpfsAddDefaults struct{}

func (t *IpfsAddDefaults) AcceptFiles() bool {
	return true
}

func (t *IpfsAddDefaults) AcceptDirs() bool {
	return false
}

func (t *IpfsAddDefaults) InstanceOptions() *utils.IpfsInstanceOptions {
	return &utils.IpfsInstanceOptions{}
}

func (t *IpfsAddDefaults) DaemonOptions() *iptb.TestEnsembleSpec {
	spec := iptb.NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "node")
	return spec
}

func (t *IpfsAddDefaults) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
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
		fmt.Println("Running against the Daemon (IPTB)")

		err := cfg.Config.ForEach(runenv, func(path string, file *os.File, isDir bool) (cid string, err error) {
			return cfg.IpfsDaemon.Add(file)
		})

		if err != nil {
			runenv.Abort(err)
			return
		}
	}

	runenv.OK()
}

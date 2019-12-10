package test

import (
	"context"
	"fmt"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
	files "github.com/ipfs/go-ipfs-files"
	coreopts "github.com/ipfs/interface-go-ipfs-core/options"
	utils "github.com/ipfs/testground/plans/chew-large-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// IpfsAddTrickleDag IPFS Add Trickle DAG Test
type IpfsAddTrickleDag struct{}

func (t *IpfsAddTrickleDag) AcceptFiles() bool {
	return true
}

func (t *IpfsAddTrickleDag) AcceptDirs() bool {
	return false
}

func (t *IpfsAddTrickleDag) InstanceOptions() *utils.IpfsInstanceOptions {
	return &utils.IpfsInstanceOptions{}
}

func (t *IpfsAddTrickleDag) DaemonOptions() *iptb.TestEnsembleSpec {
	spec := iptb.NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "node")
	return spec
}

func (t *IpfsAddTrickleDag) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
	if cfg.IpfsInstance != nil {
		fmt.Println("Running against the Core API")

		err := cfg.Config.ForEachUnixfs(runenv, func(unixfsFile files.Node, isDir bool) (string, error) {
			addOptions := func(settings *coreopts.UnixfsAddSettings) error {
				settings.Layout = coreopts.TrickleLayout
				return nil
			}

			cidFile, err := cfg.IpfsInstance.Unixfs().Add(ctx, unixfsFile, addOptions)
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
			if isDir {
				return "", fmt.Errorf("file must not be directory")
			}

			return cfg.IpfsDaemon.Add(file, func(s *shell.RequestBuilder) error {
				s.Option("trickle", true)
				return nil
			})
		})

		if err != nil {
			runenv.Abort(err)
			return
		}
	}

	runenv.OK()
}

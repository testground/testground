package test

import (
	"context"
	"fmt"
	"os"
	"time"

	utils "github.com/ipfs/testground/plans/chew-datasets/utils"
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

func (t *IpfsAddDefaults) AddRepoOptions() iptb.AddRepoOptions {
	return nil
}

func (t *IpfsAddDefaults) Execute(ctx context.Context, runenv *runtime.RunEnv, cfg *utils.TestCaseOptions) {
	if cfg.IpfsInstance != nil {
		fmt.Println("Running against the Core API")

		err := cfg.ForEachPath(runenv, func(path string, size int64, isDir bool) (string, error) {
			unixfsFile, err := utils.ConvertToUnixfs(path, isDir)
			if err != nil {
				return "", err
			}

			tstarted := time.Now()
			cidFile, err := cfg.IpfsInstance.Unixfs().Add(ctx, unixfsFile)
			if err != nil {
				return "", err
			}
			runenv.EmitMetric(utils.MakeTimeToAddMetric(size, "coreapi"), float64(time.Now().Sub(tstarted)/time.Millisecond))

			return cidFile.String(), nil
		})

		if err != nil {
			runenv.Abort(err)
			return
		}
	}

	if cfg.IpfsDaemon != nil {
		fmt.Println("Running against the Daemon (IPTB)")

		err := cfg.ForEachPath(runenv, func(path string, size int64, isDir bool) (cid string, err error) {
			file, err := os.Open(path)
			if err != nil {
				return "", err
			}

			tstarted := time.Now()
			cid, err = cfg.IpfsDaemon.Add(file)
			runenv.EmitMetric(utils.MakeTimeToAddMetric(size, "daemon"), float64(time.Now().Sub(tstarted)/time.Millisecond))
			return cid, err
		})

		if err != nil {
			runenv.Abort(err)
			return
		}
	}

	runenv.OK()
}

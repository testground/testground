package main

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/plans/chew-datasets/test"
	"github.com/ipfs/testground/plans/chew-datasets/utils"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

var testCases = []utils.TestCase{
	&test.IpfsAddDefaults{},
	&test.IpfsAddTrickleDag{},
	&test.IpfsAddDirSharding{},
	&test.IpfsMfs{},
	&test.IpfsMfsDirSharding{},
	&test.IpfsUrlStore{},
	&test.IpfsFileStore{},
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	tc := testCases[runenv.TestCaseSeq]

	cfg, err := utils.GetTestConfig(runenv, tc.AcceptFiles(), tc.AcceptDirs())
	defer cfg.Cleanup()
	if err != nil {
		runenv.Abort(fmt.Errorf("could not retrieve test config: %s", err))
		return
	}

	ctx := context.Background()

	opts := &utils.TestCaseOptions{
		IpfsInstance: nil,
		IpfsDaemon:   nil,
		TestConfig:   cfg,
	}

	mode := runenv.StringParam("mode")

	testCoreAPI := true
	testDaemon := true

	if mode != "" {
		switch mode {
		case "daemon":
			testCoreAPI = false
		case "coreapi":
			testDaemon = false
		default:
			panic(fmt.Errorf("invalid mode set: %s", mode))
		}
	}

	addRepoOptions := tc.AddRepoOptions()

	if testCoreAPI {
		ipfs, err := utils.CreateIpfsInstance(ctx, &utils.IpfsInstanceOptions{
			AddRepoOptions: addRepoOptions,
		})
		if err != nil {
			runenv.Abort(fmt.Errorf("failed to get temp dir: %s", err))
			return
		}

		opts.IpfsInstance = ipfs
	}

	if testDaemon {
		ensemble, client := iptb.SpawnDaemon(ctx, iptb.NodeOpts{
			Initialize:     true,
			Start:          true,
			AddRepoOptions: addRepoOptions,
		})
		defer ensemble.Destroy()

		opts.IpfsDaemon = client
	}

	tc.Execute(ctx, runenv, opts)
}

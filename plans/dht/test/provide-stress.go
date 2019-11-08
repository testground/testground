package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	ipfsUtil "github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/testground/sdk/runtime"
)

// ProvideStress implements the Provide Stress test case
func ProvideStress(runenv *runtime.RunEnv) {
	// Test Parameters
	var (
		timeout     = time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second
		randomWalk  = runenv.BooleanParamD("random_walk", false)
		bucketSize  = runenv.IntParamD("bucket_size", 20)
		autoRefresh = runenv.BooleanParamD("auto_refresh", true)
		nProvides   = runenv.IntParamD("n_provides", 10)
		iProvides   = time.Duration(runenv.IntParamD("i-provides", 1)) * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, dht, _, err := SetUp(ctx, runenv, timeout, randomWalk, bucketSize, autoRefresh)
	if err != nil {
		runenv.Abort(err)
		return
	}

	/// --- Act I
	// Each node calls Provide for `i-provides` until it reaches a total of `n-provides`

	var (
		seed    = 0
		counter = 0
	)

Loop:
	for {
		select {
		case <-time.After(iProvides):
			v := fmt.Sprintf("%d -- something random", seed)
			mhv := ipfsUtil.Hash([]byte(v))
			cidToPublish := cid.NewCidV0(mhv)
			err := dht.Provide(ctx, cidToPublish, true)
			if err != nil {
				runenv.Abort(fmt.Errorf("Failed on .Provide - %w", err))
				return
			}
			runenv.Message("Provided a CID")

			counter++
			if counter == nProvides {
				break Loop
			}
		case <-ctx.Done():
			runenv.Abort(fmt.Errorf("Context closed before ending the test"))
			return
		}
	}

	runenv.Message("Provided all scheduled CIDs")

	runenv.OK()
}

package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	ipfsUtil "github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// FindProviders implements the Find Providers Test case
func FindProviders(runenv *runtime.RunEnv) {
	// Test Parameters
	var (
		timeout     = time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second
		randomWalk  = runenv.BooleanParamD("random_walk", false)
		bucketSize  = runenv.IntParamD("bucket_size", 20)
		autoRefresh = runenv.BooleanParamD("auto_refresh", true)
		// pProviding  = runenv.IntParamD("p_providing", 10)
		// pResolving  = runenv.IntParamD("p_resolving", 10)
		// pFailing    = runenv.IntParamD("p_failing", 10)
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()


	_, dht, _, err := SetUp(ctx, runenv, timeout, randomWalk, bucketSize, autoRefresh, watcher, writer)
	if err != nil {
		runenv.Abort(err)
		return
	}

	/// --- Act I

	// TODO, only `p-providing` of the nodes provide a record and store its key on redis

	var (
		cidsToPublish     []cid.Cid
		nCidsToPublish    = 5
		cidsToNotPublish  []cid.Cid
		nCidsToNotPublish = 5
	)

	for i := 0; i < nCidsToPublish; i++ {
		v := fmt.Sprintf("%d -- gonna publish", i)
		mhv := ipfsUtil.Hash([]byte(v))
		cidsToPublish = append(cidsToPublish, cid.NewCidV0(mhv))
	}

	for i := 0; i < nCidsToNotPublish; i++ {
		v := fmt.Sprintf("%d -- not gonna publish", i)
		mhv := ipfsUtil.Hash([]byte(v))
		cidsToNotPublish = append(cidsToNotPublish, cid.NewCidV0(mhv))
	}

	for i := 0; i < nCidsToPublish; i++ {
		err := dht.Provide(ctx, cidsToPublish[i], true)
		if err != nil {
			runenv.Abort(fmt.Errorf("Failed on .Provide - %w", err))
			return
		}
	}

	runenv.Message("Provided a bunch of CIDs")

	/// --- Act II

	// TODO, only `p-resolving` of the nodes attempt to resolve the records provided before

	for i := 0; i < nCidsToPublish; i++ {
		peerInfos, err := dht.FindProviders(ctx, cidsToPublish[i])
		if err != nil {
			runenv.Abort(fmt.Errorf("Failed on .FindProviders - %w", err))
			return
		}
		if len(peerInfos) < 1 {
			runenv.Abort(fmt.Errorf("Should have found Providers - %w", err))
			return
		}
	}

	runenv.Message("Found a ton of providers for the CIDs I was looking for")

	// TODO, only `p-failing` of the nodes attempt to resolve records that do not exist

	for i := 0; i < nCidsToNotPublish; i++ {
		peerInfos, err := dht.FindProviders(ctx, cidsToNotPublish[i])
		if err != nil {
			runenv.Abort(err)
			return
		}
		if len(peerInfos) > 0 {
			runenv.Abort(fmt.Errorf("Should'nt have found Providers %w", err))
			return
		}
	}

	runenv.Message("Correctly didn't found providers for CIDs that are not available in the network")

	defer TearDown(ctx, runenv, watcher, writer)
	runenv.OK()
}

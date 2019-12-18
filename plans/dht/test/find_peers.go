package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func FindPeers(runenv *runtime.RunEnv) {
	opts := &SetupOpts{
		Timeout:     time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second,
		RandomWalk:  runenv.BooleanParamD("random_walk", false),
		NBootstrap:  runenv.IntParamD("n_bootstrap", 1),
		NFindPeers:  runenv.IntParamD("n_find_peers", 1),
		BucketSize:  runenv.IntParamD("bucket_size", 2),
		AutoRefresh: runenv.BooleanParamD("auto_refresh", true),
	}

	if opts.NFindPeers > runenv.TestInstanceCount {
		runenv.Abort("NFindPeers greater than the number of test instances")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	_, dht, peers, seq, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		runenv.Abort(err)
		return
	}

	defer Teardown(ctx, runenv, watcher, writer)

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ctx, runenv, watcher, writer, opts, dht, peers, seq); err != nil {
		runenv.Abort(err)
		return
	}

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, dht); err != nil {
			runenv.Abort(err)
			return
		}
	}

	// Ok, we're _finally_ ready.
	// TODO: Dump routing table stats. We should dump:
	//
	// * How full our "closest" bucket is. That is, look at the "all peers"
	//   list, find the BucketSize closest peers, and determine the % of those
	//   peers to which we're connected. It should be close to 100%.
	// * How many peers we're actually connected to?
	// * How many of our connected peers are actually useful to us?

	// Perform FIND_PEER N times.
	found := 0
	for _, p := range peers {
		if found >= opts.NFindPeers {
			break
		}
		if len(dht.Host().Peerstore().Addrs(p.ID)) > 0 {
			// Skip peer's we've already found (even if we've
			// disconnected for some reason).
			continue
		}

		t := time.Now()

		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		if _, err := dht.FindPeer(ctx, p.ID); err != nil {
			runenv.Abort(fmt.Errorf("find peer failed: %s", err))
			return
		}

		runenv.EmitMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", found),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))

		found++
	}
	runenv.OK()
}

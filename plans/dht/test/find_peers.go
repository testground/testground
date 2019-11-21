package test

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func FindPeers(runenv *runtime.RunEnv) {
	opts := &SetupOpts{
		Timeout:     time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second,
		RandomWalk:  runenv.BooleanParamD("random_walk", false),
		NFindPeers:  runenv.IntParamD("n_find_peers", 1),
		BucketSize:  runenv.IntParamD("bucket_size", 20),
		AutoRefresh: runenv.BooleanParamD("auto_refresh", true),
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	_, dht, all, _, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		runenv.Abort(err)
		return
	}

	defer Teardown(ctx, runenv, watcher, writer)

	if err = Connect(ctx, runenv, dht, opts, all...); err != nil {
		runenv.Abort(err)
		return
	}

	if err = WaitRoutingTable(ctx, runenv, dht); err != nil {
		runenv.Abort(err)
		return
	}

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, dht); err != nil {
			runenv.Abort(err)
			return
		}
	}

	// Perform FIND_PEER N times.
	attempted := make(map[peer.ID]struct{}, opts.NFindPeers)
	for i := 0; i < opts.NFindPeers; i++ {
		var next peer.ID
		for _, p := range all {
			if _, ok := attempted[p.ID]; ok {
				continue
			}
			next = p.ID
			break
		}

		if next == "" {
			// We don't have enough
			runenv.Abort(fmt.Errorf("insufficient peers in test case; FIND_PEERS iterations: %d, # peers: %d", opts.NFindPeers, len(all)))
			return
		}

		attempted[next] = struct{}{}

		t := time.Now()

		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		if _, err := dht.FindPeer(ctx, next); err != nil {
			runenv.Abort(fmt.Errorf("find peer failed: %s", err))
			return
		}

		runenv.EmitMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))

	}

	runenv.OK()
}

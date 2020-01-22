package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func FindPeers(runenv *runtime.RunEnv) error {
	opts := &SetupOpts{
		Timeout:     time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		RandomWalk:  runenv.BooleanParam("random_walk"),
		NBootstrap:  runenv.IntParam("n_bootstrap"),
		NFindPeers:  runenv.IntParam("n_find_peers"),
		BucketSize:  runenv.IntParam("bucket_size"),
		AutoRefresh: runenv.BooleanParam("auto_refresh"),
		FUndialable: runenv.FloatParam("f_undialable"),
	}

	if opts.NFindPeers > runenv.TestInstanceCount {
		return fmt.Errorf("NFindPeers greater than the number of test instances")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	node, peers, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		return err
	}

	defer Teardown(ctx, runenv, watcher, writer)

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ctx, runenv, watcher, writer, opts, node, peers); err != nil {
		return err
	}

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
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
		if len(node.host.Peerstore().Addrs(p.ID)) > 0 {
			// Skip peer's we've already found (even if we've
			// disconnected for some reason).
			continue
		}

		t := time.Now()

		ectx, cancel := context.WithCancel(ctx)
		ectx = TraceQuery(ctx, runenv, p.ID.Pretty())

		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		_, err := dht.FindPeer(ectx, p.ID)
		cancel()
		if err != nil {
			return fmt.Errorf("find peer failed: %s", err)
		}

		runenv.EmitMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", found),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))

		found++
	}
	return nil
}

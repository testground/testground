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
		ClientMode: runenv.BooleanParam("client_mode"),
	}

	if opts.NFindPeers > runenv.TestInstanceCount {
		return fmt.Errorf("NFindPeers greater than the number of test instances")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	node, peers, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		return err
	}

	defer outputGraph(node.dht, runenv, "end")
	defer Teardown(ctx, runenv, watcher, writer)

	// Bring the network into a nice, stable, bootstrapped state.
	if err = StagedBootstrap(ctx, runenv, watcher, writer, opts, node, peers); err != nil {
		return err
	}

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
		}
	}

	if err := SetupNetwork2(ctx, runenv, watcher, writer); err != nil {
		return err
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

	stg := Stager{
		ctx:     ctx,
		seq:     node.info.seq,
		total:   runenv.TestInstanceCount,
		name:    "lookup",
		stage:   0,
		watcher: watcher,
		writer:  writer,
	}

	if err := stg.Begin(); err != nil {
		return err
	}

	found := 0
	queryLog := runenv.SLogger().Named("query").With("id", node.host.ID())
	for p, info := range peers {
		if found >= opts.NFindPeers {
			break
		}
		if len(node.host.Peerstore().Addrs(p)) > 0 {
			// Skip peer's we've already found (even if we've
			// disconnected for some reason).
			continue
		}

		if _, undialable := info.properties[Undialable]; undialable {
			continue
		}

		runenv.Message("start find peer number %d", found + 1)

		ectx, cancel := outputQueryEvents(ctx, p, queryLog)

		t := time.Now()

		ectx, cancel = context.WithCancel(ctx)
		ectx = TraceQuery(ectx, runenv, p.Pretty())

		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		_, err := node.dht.FindPeer(ectx, p)
		cancel()
		if err != nil {
			_ = stg.End()
			return fmt.Errorf("find peer failed: peer %s : %s", p, err)
		}

		runenv.RecordMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", found),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Since(t).Nanoseconds()))

		found++
	}

	if err := stg.End(); err != nil {
		return err
	}

	return nil
}

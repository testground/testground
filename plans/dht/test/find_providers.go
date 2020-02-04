package test

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func FindProviders(runenv *runtime.RunEnv) error {
	opts := &SetupOpts{
		Timeout:        time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		RandomWalk:     runenv.BooleanParam("random_walk"),
		NBootstrap:     runenv.IntParam("n_bootstrap"),
		NFindPeers:     runenv.IntParam("n_find_peers"),
		BucketSize:     runenv.IntParam("bucket_size"),
		AutoRefresh:    runenv.BooleanParam("auto_refresh"),
		FUndialable:    runenv.FloatParam("f_undialable"),
		NodesProviding: runenv.IntParam("n_providing"),
		RecordCount:    runenv.IntParam("record_count"),
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
	if err = StagedBootstrap(ctx, runenv, watcher, writer, opts, node, peers); err != nil {
		return err
	}

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
		}
	}

	// Calculate the CIDs we're dealing with.
	cids := func() (out []cid.Cid) {
		for i := 0; i < opts.RecordCount; i++ {
			c := fmt.Sprintf("CID %d", i)
			out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
		}
		return out
	}()

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

	queryLog := runenv.SLogger().Named("query").With("id", node.host.ID())

	// If we're a member of the providing cohort, let's provide those CIDs to
	// the network.
	switch {
	case node.info.seq <= opts.NodesProviding:
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := outputQueryEvents(ctx, p, queryLog)
				t := time.Now()
				err := node.dht.Provide(ectx, c, true)
				cancel()
				if err == nil {
					runenv.Message("Provided CID: %s", c)
					runenv.EmitMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-provide-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Now().Sub(t).Nanoseconds()))
				}

				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stg.End()
			return fmt.Errorf("failed while providing: %s", err)
		}

	default:
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := outputQueryEvents(ctx, p, queryLog)
				t := time.Now()
				pids, err := node.dht.FindProviders(ectx, c)
				cancel()
				if err == nil {
					runenv.EmitMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-find-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Now().Sub(t).Nanoseconds()))

					runenv.EmitMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("peers-found-%d", i),
						Unit:           "peers",
						ImprovementDir: 1,
					}, float64(len(pids)))
				}

				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stg.End()
			return fmt.Errorf("failed while finding providerss: %s", err)
		}
	}

	return stg.End()
}

package test

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func FindProviders(runenv *runtime.RunEnv) {
	opts := &SetupOpts{
		Timeout:        time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second,
		RandomWalk:     runenv.BooleanParamD("random_walk", false),
		NFindPeers:     runenv.IntParamD("n_find_peers", 1),
		BucketSize:     runenv.IntParamD("bucket_size", 20),
		AutoRefresh:    runenv.BooleanParamD("auto_refresh", true),
		NodesProviding: runenv.IntParamD("nodes_providing", 10),
		RecordCount:    runenv.IntParamD("record_count", 5),
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	_, dht, _, seq, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		runenv.Abort(err)
		return
	}

	defer Teardown(ctx, runenv, watcher, writer)

	// Calculate the CIDs we're dealing with.
	cids := func() (out []cid.Cid) {
		for i := 0; i < opts.RecordCount; i++ {
			c := fmt.Sprintf("CID %d", i)
			out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
		}
		return out
	}()

	// If we're a member of the providing cohort, let's provide those CIDs to
	// the network.
	switch {
	case seq <= int64(opts.NodesProviding):
		g := errgroup.Group{}
		for i, cid := range cids {
			c := cid
			g.Go(func() error {
				t := time.Now()
				err := dht.Provide(ctx, c, true)

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
			runenv.Abort(fmt.Errorf("failed while providing: %s", err))
		} else {
			runenv.OK()
		}

	default:
		g := errgroup.Group{}
		for i, cid := range cids {
			c := cid
			g.Go(func() error {
				t := time.Now()
				pids, err := dht.FindProviders(ctx, c)

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
			runenv.Abort(fmt.Errorf("failed while finding providerss: %s", err))
		} else {
			runenv.OK()
		}
	}

}

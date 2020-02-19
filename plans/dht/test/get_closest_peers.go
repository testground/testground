package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func GetClosestPeers(runenv *runtime.RunEnv) error {
	opts := &SetupOpts{
		Timeout:        time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		RandomWalk:     runenv.BooleanParam("random_walk"),
		NFindPeers:     runenv.IntParam("n_find_peers"),
		BucketSize:     runenv.IntParam("bucket_size"),
		AutoRefresh:    runenv.BooleanParam("auto_refresh"),
		FUndialable:    runenv.FloatParam("f_undialable"),
		ClientMode:     runenv.BooleanParam("client_mode"),
		NDisjointPaths: runenv.IntParam("n_paths"),
		Datastore:      runenv.IntParam("datastore"),
		RecordCount:    runenv.IntParam("record_count"),
		Debug:          runenv.IntParam("dbg"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	node, peers, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		return err
	}

	defer Teardown(ctx, runenv, watcher, writer)

	//if err := testBarrier(ctx, runenv, watcher, writer, node.info.seq); err != nil {
	//	return err
	//}
	//return nil

	t := time.Now()

	// Bring the network into a nice, stable, bootstrapped state.
	if err = StagedBootstrap(ctx, runenv, watcher, writer, opts, node, peers); err != nil {
		return err
	}

	runenv.RecordMetric(&runtime.MetricDefinition{
		Name:           fmt.Sprintf("bs"),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	if opts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
		}
	}

	t = time.Now()

	if err := SetupNetwork2(ctx, runenv, watcher, writer); err != nil {
		return err
	}

	runenv.RecordMetric(&runtime.MetricDefinition{
		Name:           fmt.Sprintf("reset"),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	t = time.Now()

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
		re:      runenv,
	}

	isFinder := node.info.seq < opts.NFindPeers

	if err := stg.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start gcp loop")
	runenv.RecordMessage(fmt.Sprintf("isFinder: %v, seqNo: %v, numFPeers %d, numRecords: %d", isFinder, node.info.seq, opts.NFindPeers, len(cids)))

	if isFinder {
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := context.WithCancel(ctx)
				ectx = TraceQuery(ctx, runenv, node, p.Pretty())
				t := time.Now()
				pids, err := node.dht.GetClosestPeers(ectx, c.KeyString())
				cancel()

				peers := make([]peer.ID, 0, opts.BucketSize)
				for p := range pids {
					peers = append(peers, p)
				}

				if err == nil {
					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-find-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("peers-found-%d", i),
						Unit:           "peers",
						ImprovementDir: 1,
					}, float64(len(pids)))

					outputGCP(runenv, node.info.addrs.ID, c, peers)
				} else {
					runenv.RecordMessage("Error during GCP %w", err)
				}
				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stg.End()
			return fmt.Errorf("failed while finding providerss: %s", err)
		}
	}

	runenv.RecordMessage("done provide loop")

	if err := stg.End(); err != nil {
		return err
	}

	runenv.RecordMetric(&runtime.MetricDefinition{
		Name:           fmt.Sprintf("search"),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	outputGraph(node.dht, "end")

	return nil
}

func outputGCP(runenv *runtime.RunEnv, me peer.ID, target cid.Cid, peers []peer.ID) {
	peerStrs := make([]string, len(peers))
	kadPeerStrs := make([]string, len(peers))

	for i, p := range peers {
		peerStrs[i] = p.String()
		kadPeerStrs[i] = hex.EncodeToString(kbucket.ConvertKey(string(p)))
	}

	nodeLogger.Infow("gcp-results",
		"me", me.String(),
		"KadMe", hex.EncodeToString(kbucket.ConvertKey(string(me))),
		"target", target.String(),
		"peers", peerStrs,
		"KadTarget", hex.EncodeToString(kbucket.ConvertKey(target.KeyString())),
		"KadPeers", kadPeerStrs,
	)
	nodeLogger.Sync()
}

func testBarrier(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, seq int) error {
	stg0 := Stager{
		ctx:     ctx,
		seq:     seq,
		total:   runenv.TestInstanceCount,
		name:    "tester",
		stage:   0,
		watcher: watcher,
		writer:  writer,
		re:      runenv,
	}

	for i := 0; i < 100; i++ {
		stg0.Begin()
		t := time.Now()
		err := stg0.End()
		if err != nil {
			return err
		}
		runenv.RecordMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("stage-time"),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Since(t).Nanoseconds()))
	}
	return nil
}

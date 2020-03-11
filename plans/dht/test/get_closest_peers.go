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
	opts := GetCommonOpts(runenv)
	opts.RecordCount = runenv.IntParam("record_count")
	opts.Debug = runenv.IntParam("dbg")

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

	stager := NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "default", watcher, writer, runenv)

	t := time.Now()

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ctx, runenv, opts, node, peers, stager, GetBootstrapNodes(opts, node, peers)); err != nil {
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

	if err := SetupNetwork(ctx, runenv, watcher, writer, 100*time.Millisecond); err != nil {
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

	isFinder := node.info.Seq < opts.NFindPeers

	stager.Reset("lookup")
	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start gcp loop")
	runenv.RecordMessage(fmt.Sprintf("isFinder: %v, seqNo: %v, numFPeers %d, numRecords: %d", isFinder, node.info.Seq, opts.NFindPeers, len(cids)))

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

					outputGCP(runenv, node.info.Addrs.ID, c, peers)
				} else {
					runenv.RecordMessage("Error during GCP %w", err)
				}
				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stager.End()
			return fmt.Errorf("failed while finding providerss: %s", err)
		}
	}

	runenv.RecordMessage("done provide loop")

	if err := stager.End(); err != nil {
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

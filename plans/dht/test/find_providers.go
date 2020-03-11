package test

import (
	"context"
	"fmt"
	"math/bits"
	"reflect"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type findProvsParams struct {
	RecordSeed      int
	RecordCount int
	SearchRecords bool
}

func FindProviders(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)
	fpOpts := findProvsParams{
		RecordSeed: runenv.IntParam("record_seed"),
		RecordCount: runenv.IntParam("record_count"),
		SearchRecords: runenv.BooleanParam("search_records"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	ri := &RunInfo{
		runenv:  runenv,
		watcher: watcher,
		writer:  writer,
	}

	node, peers, err := Setup(ctx, ri, commonOpts)
	if err != nil {
		return err
	}

	defer Teardown(ctx, runenv, watcher, writer)

	stager := NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "default", watcher, writer, runenv)

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ctx, runenv, commonOpts, node, peers, stager, GetBootstrapNodes(commonOpts, node, peers)); err != nil {
		return err
	}

	if commonOpts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
		}
	}

	if err := SetupNetwork(ctx, ri, 100*time.Millisecond); err != nil {
		return err
	}

	stager.Reset("lookup")
	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start provide loop")

	// If we're a member of the providing cohort, let's provide those CIDs to
	// the network.
	if fpOpts.RecordCount > 0 {
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := context.WithCancel(ctx)
				ectx = TraceQuery(ctx, runenv, node, p.Pretty())
				t := time.Now()
				err := node.dht.Provide(ectx, c, true)
				cancel()
				if err == nil {
					runenv.RecordMessage("Provided CID: %s", c)
					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-provide-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))
				}

				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stager.End()
			return fmt.Errorf("failed while providing: %s", err)
		}
	}

	if err := stager.End(); err != nil {
		return err
	}

	if err := stager.Begin(); err != nil {
		return err
	}

	if fpOpts.SearchRecords {
		g := errgroup.Group{}
		for index, cid := range cids {
			i := index
			c := cid
			g.Go(func() error {
				p := peer.ID(c.Bytes())
				ectx, cancel := context.WithCancel(ctx)
				ectx = TraceQuery(ctx, runenv, node, p.Pretty())
				t := time.Now()
				pids, err := node.dht.FindProviders(ectx, c)
				cancel()
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
				}

				return err
			})
		}

		if err := g.Wait(); err != nil {
			_ = stager.End()
			return fmt.Errorf("failed while finding providerss: %s", err)
		}
	}

	if err := stager.End(); err != nil {
		return err
	}

	outputGraph(node.dht, "end")

	return nil
}

func get(ctx context.Context, ri *RunInfo, info *NodeInfo, fpOpts findProvsParams) ([]cid.Cid, []cid.Cid, error) {
	var emitRecords, searchRecords []cid.Cid
	if fpOpts.RecordCount > 0 {
		// Calculate the CIDs we're dealing with.
		emitRecords = func() (out []cid.Cid) {
			for i := 0; i < fpOpts.RecordCount; i++ {
				c := fmt.Sprintf("CID %d - group %s - seeded with %d", i, ri.runenv.TestGroupID, fpOpts.RecordSeed)
				out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
			}
			return out
		}()
	}
	
	var recordPayload *RecordSubmission
	if info.GroupSeq == 0 {
		recordPayload = &RecordSubmission{
			RecordIDs: emitRecords,
			GroupID:   ri.runenv.TestGroupID,
		}
	}

	err := syncAll(ctx, ri, len(emitRecords), RecordsSubtree, recordPayload,
		func(v interface{}) {
			searchRecords = append(searchRecords, v.(*RecordSubmission).RecordIDs...)
		},
		)
	if err != nil {
		return nil, nil, err
	}

	return emitRecords, searchRecords, nil
}

// PeerAttribSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var RecordsSubtree = &sync.Subtree{
	GroupKey:    "records",
	PayloadType: reflect.TypeOf((*RecordSubmission)(nil)),
	KeyFunc: func(val interface{}) string {
		return val.(*RecordSubmission).GroupID
	},
}

type RecordSubmission struct {
	RecordIDs []cid.Cid
	GroupID string
}
package test

import (
	"context"
	"fmt"
	u "github.com/ipfs/go-ipfs-util"
	"reflect"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type findProvsParams struct {
	RecordSeed    int
	RecordCount   int
	SearchRecords bool
}

func FindProviders(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)
	fpOpts := findProvsParams{
		RecordSeed:    runenv.IntParam("record_seed"),
		RecordCount:   runenv.IntParam("record_count"),
		SearchRecords: runenv.BooleanParam("search_records"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	//defer watcher.Close()
	//defer writer.Close()

	runenv.RecordMessage("New test1")

	ri := &RunInfo{
		runenv:  runenv,
		watcher: watcher,
		writer:  writer,
	}
	
	z := SetupNetwork(ctx, ri, 0)
	if z != nil {
		return z
	}
	return nil

	node, peers, err := Setup(ctx, ri, commonOpts)
	if err != nil {
		return err
	}

	defer Teardown(ctx, ri)

	stager := NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "default", ri)

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

	runenv.RecordMessage("start get records")

	emitRecords, searchRecords, err := getRecords(ctx, ri, node.info, fpOpts)
	if err != nil {
		return err
	}

	if err := stager.End(); err != nil {
		return err
	}

	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start provide loop")

	// If we're a member of the providing cohort, let's provide those CIDs to
	// the network.
	if fpOpts.RecordCount > 0 {
		g := errgroup.Group{}
		for index, cid := range emitRecords {
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
		for _, record := range searchRecords {
			for index, cid := range record.RecordIDs {
				i := index
				c := cid
				groupID := record.GroupID
				g.Go(func() error {
					p := peer.ID(c.Bytes())
					ectx, cancel := context.WithCancel(ctx)
					ectx = TraceQuery(ctx, runenv, node, p.Pretty())
					t := time.Now()
					pids, err := node.dht.FindProviders(ectx, c)
					cancel()
					if err == nil {
						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("time-to-find|%s|%d", groupID, i),
							Unit:           "ns",
							ImprovementDir: -1,
						}, float64(time.Since(t).Nanoseconds()))

						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("peers-found|%s|%d", groupID, i),
							Unit:           "peers",
							ImprovementDir: 1,
						}, float64(len(pids)))
					}

					return err
				})
			}
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

// getRecords returns the records we plan to store and those we plan to search for. It also tells other nodes via the
// sync service which nodes our group plans on advertising
func getRecords(ctx context.Context, ri *RunInfo, info *NodeInfo, fpOpts findProvsParams) ([]cid.Cid, []*RecordSubmission, error) {
	recGen := func(group string) (out []cid.Cid) {
		for i := 0; i < fpOpts.RecordCount; i++ {
			c := fmt.Sprintf("CID %d - group %s - seeded with %d", i, group, fpOpts.RecordSeed)
			out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
		}
		return out
	}

	var emitRecords []cid.Cid
	if fpOpts.RecordCount > 0 {
		// Calculate the CIDs we're dealing with.
		emitRecords = recGen(ri.runenv.TestGroupID)
	}

	if info.GroupSeq == 0 {
		ri.runenv.RecordMessage("writing records")
		record := &RecordSubmission{
			RecordIDs: emitRecords,
			GroupID:   ri.runenv.TestGroupID,
		}
		if _, err := ri.writer.Write(ctx, RecordsSubtree, record); err != nil {
			return nil, nil, err
		}
	}

	var searchRecords []*RecordSubmission
	if fpOpts.SearchRecords {
		ri.runenv.RecordMessage("getting records")

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		recordCh := make(chan *RecordSubmission)
		if err := ri.watcher.Subscribe(subCtx, RecordsSubtree, recordCh); err != nil {
			return nil, nil, err
		}

		for i := 0; i < len(ri.groups); i++ {
			select {
			case rec := <-recordCh:
				if len(rec.RecordIDs) > 0 {
					searchRecords = append(searchRecords, rec)
				}
			case <-time.After(time.Second * 5):
				ri.runenv.RecordMessage("haaaallp")
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
	}

	ri.runenv.RecordMessage("finished recfn")

	return emitRecords, searchRecords, nil
}

// RecordsSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise the records their group will put into the DHT.
var RecordsSubtree = &sync.Subtree{
	GroupKey:    "records",
	PayloadType: reflect.TypeOf(&RecordSubmission{}),
	KeyFunc: func(val interface{}) string {
		return val.(*RecordSubmission).GroupID
	},
}

type RecordSubmission struct {
	RecordIDs []cid.Cid
	GroupID   string
}

package test

import (
	"context"
	"fmt"
	"github.com/ipfs/testground/plans/dht/utils"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/testground/sdk/runtime"
)

type findProvsParams struct {
	RecordSeed    int
	RecordCount   int
	SearchRecords bool
}

func getFindProvsParams(params map[string]string) findProvsParams {
	tmpRunEnv := runtime.RunEnv{RunParams: runtime.RunParams{
		TestInstanceParams: params,
	}}

	fpOpts := findProvsParams{
		RecordSeed:    tmpRunEnv.IntParam("record_seed"),
		RecordCount:   tmpRunEnv.IntParam("record_count"),
		SearchRecords: tmpRunEnv.BooleanParam("search_records"),
	}

	return fpOpts
}

func FindProviders(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	ri, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	if err := TestProviderRecords(ctx, ri); err != nil {
		return err
	}
	Teardown(ctx, ri.RunInfo)

	return nil
}

func TestProviderRecords(ctx context.Context, ri *DHTRunInfo) error {
	runenv := ri.RunEnv
	node := ri.Node

	fpOpts := getFindProvsParams(ri.RunEnv.RunParams.TestInstanceParams)

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "provider-records", ri.RunInfo)

	emitRecords, searchRecords := getRecords(ri, fpOpts)

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
				ectx = TraceQuery(ctx, runenv, node, p.Pretty(), "provider-records")
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

	outputGraph(node.dht, "after_provide")

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
					ectx = TraceQuery(ctx, runenv, node, p.Pretty(), "provider-records")
					t := time.Now()

					numProvs := 0
					provsCh := node.dht.FindProvidersAsync(ectx, c, getAllProvRecordsNum())
					status := "done"

					var tLastFound time.Time
				provLoop:
					for {
						select {
						case _, ok := <-provsCh:
							if !ok {
								break provLoop
							}

							tLastFound = time.Now()

							if numProvs == 0 {
								runenv.RecordMetric(&runtime.MetricDefinition{
									Name:           fmt.Sprintf("time-to-find-first|%s|%d", groupID, i),
									Unit:           "ns",
									ImprovementDir: -1,
								}, float64(tLastFound.Sub(t).Nanoseconds()))
							}

							numProvs++
						case <-ctx.Done():
							status = "incomplete"
							break provLoop
						}
					}
					cancel()

					if numProvs > 0 {
						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("time-to-find-last|%s|%s|%d", status, groupID, i),
							Unit:           "ns",
							ImprovementDir: -1,
						}, float64(tLastFound.Sub(t).Nanoseconds()))
					} else if status != "incomplete" {
						status = "fail"
					}

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-find|%s|%s|%d", status, groupID, i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("peers-found|%s|%s|%d", status, groupID, i),
						Unit:           "peers",
						ImprovementDir: 1,
					}, float64(numProvs))

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("peers-missing|%s|%s|%d", status, groupID, i),
						Unit:           "peers",
						ImprovementDir: -1,
					}, float64(ri.GroupProperties[groupID].Size-numProvs))

					return nil
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
	return nil
}

// getRecords returns the records we plan to store and those we plan to search for. It also tells other nodes via the
// sync service which nodes our group plans on advertising
func getRecords(ri *DHTRunInfo, fpOpts findProvsParams) ([]cid.Cid, []*ProviderRecordSubmission) {
	recGen := func(groupID string, groupFPOpts findProvsParams) (out []cid.Cid) {
		for i := 0; i < groupFPOpts.RecordCount; i++ {
			c := fmt.Sprintf("CID %d - group %s - seeded with %d", i, groupID, groupFPOpts.RecordSeed)
			out = append(out, cid.NewCidV0(u.Hash([]byte(c))))
		}
		return out
	}

	var emitRecords []cid.Cid
	if fpOpts.RecordCount > 0 {
		// Calculate the CIDs we're dealing with.
		emitRecords = recGen(ri.Node.info.Group, fpOpts)
	}

	var searchRecords []*ProviderRecordSubmission
	if fpOpts.SearchRecords {
		for _, g := range ri.Groups {
			gOpts := ri.GroupProperties[g]
			groupFPOpts := getFindProvsParams(gOpts.Params)
			if groupFPOpts.RecordCount > 0 {
				searchRecords = append(searchRecords, &ProviderRecordSubmission{
					RecordIDs: recGen(g, groupFPOpts),
					GroupID:   g,
				})
			}
		}
	}

	return emitRecords, searchRecords
}

type ProviderRecordSubmission struct {
	RecordIDs []cid.Cid
	GroupID   string
}

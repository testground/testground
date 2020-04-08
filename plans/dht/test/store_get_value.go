package test

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ipfs/testground/plans/dht/utils"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ipfs/go-ipns"

	"golang.org/x/sync/errgroup"
)

type storeGetParams struct {
	RecordSeed    int
	RecordCount   int
	SearchRecords bool
}

func StoreGetValue(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	ri, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	if err := TestIPNSRecords(ctx, ri); err != nil {
		return err
	}
	Teardown(ctx, ri.RunInfo)

	return nil
}

func TestIPNSRecords(ctx context.Context, ri *DHTRunInfo) error {
	runenv := ri.RunEnv
	node := ri.Node

	fpOpts := getFindProvsParams(ri.RunEnv.RunParams.TestInstanceParams)

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "default", ri.RunInfo)

	stager.Reset("lookup")

	emitRecords, searchRecords, err := getIPNSRecords(ri, fpOpts)
	if err != nil {
		return err
	}
	emitRecordsKeys := make([]string, len(emitRecords))
	for i, privKey := range emitRecords {
		pid, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			return err
		}
		emitRecordsKeys[i] = ipns.RecordKey(pid)
	}

	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("start provide loop")

	// If we're a member of the providing cohort, let's provide those CIDs to
	// the network.
	if fpOpts.RecordCount > 0 {
		g := errgroup.Group{}
		for index, privKey := range emitRecords {
			i := index
			record, err := ipns.Create(privKey, []byte("/path/to/stuff"), 0, time.Now().Add(time.Hour))
			if err != nil {
				return err
			}
			if err := ipns.EmbedPublicKey(privKey.GetPublic(), record); err != nil {
				return err
			}
			recordKey := emitRecordsKeys[i]
			recordBytes, err := record.Marshal()
			if err != nil {
				return err
			}
			g.Go(func() error {
				ectx, cancel := context.WithCancel(ctx)
				ectx = TraceQuery(ctx, runenv, node, recordKey)
				t := time.Now()
				err := node.dht.PutValue(ectx, recordKey, recordBytes)
				cancel()
				if err == nil {
					runenv.RecordMessage("Provided IPNS Key: %s", recordKey)
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
			for index, key := range record.RecordIDs {
				i := index
				k := key
				groupID := record.GroupID
				g.Go(func() error {
					ectx, cancel := context.WithCancel(ctx)
					ectx = TraceQuery(ctx, runenv, node, k)
					t := time.Now()

					numProvs := 0
					provsCh, err := node.dht.SearchValue(ectx, k)
					if err != nil {
						return err
					}
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

// getIPNSRecords returns the records we plan to store and those we plan to search for. It also tells other nodes via the
// sync service which nodes our group plans on advertising
func getIPNSRecords(ri *DHTRunInfo, fpOpts findProvsParams) (emitRecords []crypto.PrivKey, searchRecords []*RecordSubmission, err error) {
	recGen := func(groupID string, groupFPOpts findProvsParams) (out []crypto.PrivKey, err error) {
		rng := rand.New(rand.NewSource(int64(fpOpts.RecordSeed)))
		for _, g := range ri.Groups {
			rng.Int63()
			if g == groupID {
				break
			}
		}
		for i := 0; i < groupFPOpts.RecordCount; i++ {
			priv, _, err := crypto.GenerateEd25519Key(rng)
			if err != nil {
				return nil, err
			}
			out = append(out, priv)
		}
		return out, nil
	}

	if fpOpts.RecordCount > 0 {
		// Calculate the CIDs we're dealing with.
		emitRecords, err = recGen(ri.Node.info.Group, fpOpts)
		if err != nil {
			return
		}
	}

	if fpOpts.SearchRecords {
		for _, g := range ri.Groups {
			gOpts := ri.GroupProperties[g]
			groupFPOpts := getFindProvsParams(gOpts.Params)
			if groupFPOpts.RecordCount > 0 {
				recs, err := recGen(g, groupFPOpts)
				if err != nil {
					return nil, nil, err
				}
				ipnsKeys := make([]string, len(recs))
				for i, k := range recs {
					pid, err := peer.IDFromPrivateKey(k)
					if err != nil {
						return nil, nil, err
					}
					ipnsKeys[i] = ipns.RecordKey(pid)
				}
				searchRecords = append(searchRecords, &RecordSubmission{
					RecordIDs: ipnsKeys,
					GroupID:   g,
				})
			}
		}
	}

	return
}

type RecordSubmission struct {
	RecordIDs []string
	GroupID   string
}

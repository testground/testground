package test

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/testground/testground/plans/dht/utils"
	"github.com/testground/testground/sdk/runtime"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipns"
	ipns_pb "github.com/ipfs/go-ipns/pb"

	"golang.org/x/sync/errgroup"
)

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

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "ipns-records", ri.RunInfo)

	emitRecords, searchRecords, err := generateIPNSRecords(ri, fpOpts)
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

	runenv.RecordMessage("start put loop")

	for i := 0; i < 5; i++ {
		if err := putIPNSRecord(ctx, ri, fpOpts, stager, i, emitRecords, emitRecordsKeys); err != nil {
			return err
		}

		if err := getIPNSRecord(ctx, ri, fpOpts, stager, i, searchRecords); err != nil {
			return err
		}
	}
	return nil
}

func putIPNSRecord(ctx context.Context, ri *DHTRunInfo, fpOpts findProvsParams, stager utils.Stager, recNum int, emitRecords []crypto.PrivKey, emitRecordsKeys []string) error {
	runenv := ri.RunEnv
	node := ri.Node

	if err := stager.Begin(); err != nil {
		return err
	}

	// If we're a member of the putting cohort, let's put those IPNS records to the network.
	if fpOpts.RecordCount > 0 {
		g := errgroup.Group{}
		for index, privKey := range emitRecords {
			i := index
			record, err := ipns.Create(privKey, []byte(fmt.Sprintf("/path/to/stuff/%d", recNum)), uint64(recNum), time.Now().Add(time.Hour))
			if err != nil {
				return err
			}
			if err := ipns.EmbedPublicKey(privKey.GetPublic(), record); err != nil {
				return err
			}
			recordKey := emitRecordsKeys[i]
			recordBytes, err := proto.Marshal(record)
			if err != nil {
				return err
			}
			g.Go(func() error {
				ectx, cancel := context.WithCancel(ctx) //nolint
				ectx = TraceQuery(ctx, runenv, node, recordKey, "ipns-records")
				t := time.Now()
				err := node.dht.PutValue(ectx, recordKey, recordBytes)
				cancel()
				if err == nil {
					runenv.RecordMessage("Put IPNS Key: %s", recordKey)
					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-put-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))
				} else {
					runenv.RecordMessage("Failed to Put IPNS Key: %s : err: %s", recordKey, err)
					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-failed-put-%d", i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))
				}

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			panic("how is there an error here?")
		}
	}

	if err := stager.End(); err != nil {
		return err
	}
	return nil
}

func getIPNSRecord(ctx context.Context, ri *DHTRunInfo, fpOpts findProvsParams, stager utils.Stager, recNum int, searchRecords []*RecordSubmission) error {
	runenv := ri.RunEnv
	node := ri.Node

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
					ectx, cancel := context.WithCancel(ctx) //nolint
					ectx = TraceQuery(ctx, runenv, node, k, "ipns-records")
					t := time.Now()

					runenv.RecordMessage("Searching for IPNS Key: %s", k)
					numRecs := 0
					recordCh, err := node.dht.SearchValue(ectx, k)
					if err != nil {
						runenv.RecordMessage("Failed to Search for IPNS Key: %s : err: %s", k, err)
						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("time-to-failed-put-%d", i),
							Unit:           "ns",
							ImprovementDir: -1,
						}, float64(time.Since(t).Nanoseconds()))
						return nil //nolint
					}
					status := "done"

					var tLastFound time.Time
					var lastRec []byte
				searchLoop:
					for {
						select {
						case rec, ok := <-recordCh:
							if !ok {
								break searchLoop
							}
							lastRec = rec

							tLastFound = time.Now()

							if numRecs == 0 {
								runenv.RecordMetric(&runtime.MetricDefinition{
									Name:           fmt.Sprintf("time-to-get-first|%s|%d", groupID, i),
									Unit:           "ns",
									ImprovementDir: -1,
								}, float64(tLastFound.Sub(t).Nanoseconds()))
							}

							numRecs++
						case <-ctx.Done():
							break searchLoop
						}
					}
					cancel()

					if numRecs > 0 {
						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("time-to-get-last|%s|%s|%d", status, groupID, i),
							Unit:           "ns",
							ImprovementDir: -1,
						}, float64(tLastFound.Sub(t).Nanoseconds()))

						runenv.RecordMetric(&runtime.MetricDefinition{
							Name:           fmt.Sprintf("record-updates|%s|%s|%d|%d", status, groupID, recNum, i),
							Unit:           "records",
							ImprovementDir: -1,
						}, float64(numRecs))

						if len(lastRec) == 0 {
							panic("this should not be possible")
						}

						recordResult := &ipns_pb.IpnsEntry{}
						if err := recordResult.Unmarshal(lastRec); err != nil {
							panic(fmt.Errorf("received invalid IPNS record: err %v", err))
						}

						if diff := int(*recordResult.Sequence) - recNum; diff > 0 {
							runenv.RecordMetric(&runtime.MetricDefinition{
								Name:           fmt.Sprintf("incomplete-get|%s|%d|%d", groupID, recNum, i),
								Unit:           "records",
								ImprovementDir: -1,
							}, float64(diff))
							status = "fail"
						}

					} else {
						status = "fail"
					}

					runenv.RecordMetric(&runtime.MetricDefinition{
						Name:           fmt.Sprintf("time-to-get|%s|%s|%d|%d", status, groupID, recNum, i),
						Unit:           "ns",
						ImprovementDir: -1,
					}, float64(time.Since(t).Nanoseconds()))

					return nil
				})
			}
		}

		if err := g.Wait(); err != nil {
			panic("how is this possible?")
		}
	}

	if err := stager.End(); err != nil {
		return err
	}
	return nil
}

// generateIPNSRecords returns the records we plan to store and those we plan to search for
func generateIPNSRecords(ri *DHTRunInfo, fpOpts findProvsParams) (emitRecords []crypto.PrivKey, searchRecords []*RecordSubmission, err error) {
	recGen := func(groupID string, groupFPOpts findProvsParams) (out []crypto.PrivKey, err error) {
		// Calculate key based on seed
		rng := rand.New(rand.NewSource(int64(fpOpts.RecordSeed)))
		// Calculate key based on group (run through the rng to do this)
		for _, g := range ri.Groups {
			rng.Int63()
			if g == groupID {
				break
			}
		}
		// Unique key per record is generated since the rng is mutated by creating the new key
		for i := 0; i < groupFPOpts.RecordCount; i++ {
			priv, _, err := crypto.GenerateEd25519Key(rng)
			if err != nil {
				return nil, err
			}
			out = append(out, priv)
		}
		return out, nil
	}

	if fpOpts.RecordCount > 0 && ri.Node.info.GroupSeq == 0 {
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

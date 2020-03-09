package test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/testground/plans/bitswap-tuning/utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// NOTE: To run use:
// ./testground run data-exchange/transfer --builder=docker:go --runner="local:docker" --dep="github.com/ipfs/go-bitswap=master"

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv) error {
	// Test Parameters
	timeout := time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	runTimeout := time.Duration(runenv.IntParam("run_timeout_secs")) * time.Second
	parallelGenMax := runenv.IntParam("parallel_gen_mb") * 1024 * 1024
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")
	seedCount := runenv.TestInstanceCount - (leechCount + passiveCount)
	requestStagger := time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond
	bstoreDelay := time.Duration(runenv.IntParam("bstore_delay_ms")) * time.Millisecond
	runCount := runenv.IntParam("run_count")
	fileSizes, err := utils.ParseIntArray(runenv.StringParam("file_size"))
	if err != nil {
		return err
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)

	/// --- Tear down
	defer func() {
		err := utils.SignalAndWaitForAll(ctx, runenv.TestInstanceCount, "end", watcher, writer)
		if err != nil {
			runenv.RecordFailure(err)
		} else {
			runenv.RecordSuccess()
		}
		watcher.Close()
		writer.Close()
	}()

	// Create libp2p node
	h, err := libp2p.New(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	// Get sequence number of this host
	seq, err := writer.Write(ctx, sync.PeerSubtree, host.InfoFromHost(h))
	if err != nil {
		return err
	}

	// Get addresses of all peers
	peerCh := make(chan *peer.AddrInfo)
	sctx, cancelSub := context.WithCancel(ctx)
	if err := watcher.Subscribe(sctx, sync.PeerSubtree, peerCh); err != nil {
		cancelSub()
		return err
	}
	addrInfos, err := utils.AddrInfosFromChan(peerCh, runenv.TestInstanceCount)
	if err != nil {
		cancelSub()
		return fmt.Errorf("no addrs in %d seconds", timeout/time.Second)
	}
	cancelSub()

	/// --- Warm up

	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Note: seq starts at 1 (not 0)
	var nodetp utils.NodeType
	var tpindex int
	switch {
	case seq <= int64(leechCount):
		nodetp = utils.Leech
		tpindex = int(seq) - 1
		runenv.RecordMessage("I am leech %d", tpindex)
	case seq > int64(leechCount+passiveCount):
		nodetp = utils.Seed
		tpindex = int(seq) - 1 - (leechCount + passiveCount)
		runenv.RecordMessage("I am seed %d", tpindex)
	default:
		nodetp = utils.Passive
		tpindex = int(seq) - 1 - leechCount
		runenv.RecordMessage("I am passive node %d (neither leech nor seed)", tpindex)
	}

	// Set up network (with traffic shaping)
	latency, bandwidthMB, err := utils.SetupNetwork(ctx, runenv, watcher, writer, nodetp, tpindex)
	if err != nil {
		return fmt.Errorf("Failed to set up network: %w", err)
	}

	// Use the same blockstore on all runs for the seed node
	var bstore blockstore.Blockstore
	if nodetp == utils.Seed {
		bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
		if err != nil {
			return err
		}
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		return utils.SignalAndWaitForAll(ctx, runenv.TestInstanceCount, state, watcher, writer)
	}

	// For each file size
	for sizeIndex, fileSize := range fileSizes {
		// If the total amount of seed data to be generated is greater than
		// parallelGenMax, generate seed data in series
		genSeedSerial := seedCount > 2 || fileSize*seedCount > parallelGenMax

		// Run the test runCount times
		var rootCid cid.Cid
		for runNum := 1; runNum < runCount+1; runNum++ {
			// Reset the timeout for each run
			ctx, cancel := context.WithTimeout(ctx, runTimeout)
			defer cancel()

			isFirstRun := runNum == 1
			runId := fmt.Sprintf("%d-%d", sizeIndex, runNum)

			// Wait for all nodes to be ready to start the run
			err = signalAndWaitForAll("start-run-" + runId)
			if err != nil {
				return err
			}

			runenv.RecordMessage("Starting run %d / %d (%d bytes)", runNum, runCount, fileSize)
			var bsnode *utils.Node
			rootCidSubtree := getRootCidSubtree(sizeIndex)

			switch nodetp {
			case utils.Seed:
				// For seeds, create a new bitswap node from the existing datastore
				bsnode, err = utils.CreateBitswapNode(ctx, h, bstore)
				if err != nil {
					return err
				}

				// If this is the first run for this file size
				if isFirstRun {
					seedGenerated := sync.State("seed-generated-" + runId)
					var start time.Time
					if genSeedSerial {
						// Each seed generates the seed data in series, to avoid
						// overloading a single machine hosting multiple instances
						seedIndex := seq - int64(leechCount+passiveCount) - 1

						if seedIndex > 0 {
							// Wait for the seeds with an index lower than this one
							// to generate their seed data
							doneCh := watcher.Barrier(ctx, seedGenerated, int64(seedIndex))
							if err = <-doneCh; err != nil {
								return err
							}
						}

						// Generate a file of the given size and add it to the datastore
						runenv.RecordMessage("Generating seed data of %d bytes", fileSize)
						start = time.Now()
					}

					rootCid, err := setupSeed(ctx, bsnode, fileSize)
					if err != nil {
						return fmt.Errorf("Failed to set up seed: %w", err)
					}

					if genSeedSerial {
						runenv.RecordMessage("Done generating seed data of %d bytes (%s)", fileSize, time.Since(start))

						// Signal we've completed generating the seed data
						_, err = writer.SignalEntry(ctx, seedGenerated)
						if err != nil {
							return fmt.Errorf("Failed to signal seed generated: %w", err)
						}
					}

					// Inform other nodes of the root CID
					if _, err = writer.Write(ctx, rootCidSubtree, &rootCid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidSubtree %w", err)
					}
				}
			case utils.Leech:
				// For leeches, create a new blockstore on each run
				bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
				if err != nil {
					return err
				}

				// Create a new bitswap node from the blockstore
				bsnode, err = utils.CreateBitswapNode(ctx, h, bstore)
				if err != nil {
					return err
				}

				// If this is the first run for this file size
				if isFirstRun {
					// Get the root CID from a seed
					rootCidCh := make(chan *cid.Cid, 1)
					sctx, cancelRootCidSub := context.WithCancel(ctx)
					if err := watcher.Subscribe(sctx, rootCidSubtree, rootCidCh); err != nil {
						cancelRootCidSub()
						return fmt.Errorf("Failed to subscribe to rootCidSubtree %w", err)
					}

					// Note: only need to get the root CID from one seed - it should be the
					// same on all seeds (seed data is generated from repeatable random
					// sequence)
					rootCidPtr, ok := <-rootCidCh
					cancelRootCidSub()
					if !ok {
						return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
					}
					rootCid = *rootCidPtr
				}
			}

			// Wait for all nodes to be ready to dial
			err = signalAndWaitForAll("ready-to-connect-" + runId)
			if err != nil {
				return err
			}

			// Dial all peers
			dialed, err := utils.DialOtherPeers(ctx, h, addrInfos)
			if err != nil {
				return err
			}
			runenv.RecordMessage("Dialed %d other nodes", len(dialed))

			// Wait for all nodes to be connected
			err = signalAndWaitForAll("connect-complete-" + runId)
			if err != nil {
				return err
			}

			/// --- Start test

			var timeToFetch time.Duration
			if nodetp == utils.Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				startDelay := time.Duration(seq-1) * requestStagger
				time.Sleep(startDelay)

				runenv.RecordMessage("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				err := bsnode.FetchGraph(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					return fmt.Errorf("Error fetching data through Bitswap: %w", err)
				}
				runenv.RecordMessage("Leech fetch complete (%s)", timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			err = signalAndWaitForAll("transfer-complete-" + runId)
			if err != nil {
				return err
			}

			/// --- Report stats
			err = emitMetrics(runenv, bsnode, runNum, seq, latency, bandwidthMB, fileSize, nodetp, tpindex, timeToFetch)
			if err != nil {
				return err
			}

			// Shut down bitswap
			err = bsnode.Close()
			if err != nil {
				return fmt.Errorf("Error closing Bitswap: %w", err)
			}

			// Disconnect peers
			for _, c := range h.Network().Conns() {
				err := c.Close()
				if err != nil {
					return fmt.Errorf("Error disconnecting: %w", err)
				}
			}

			if nodetp == utils.Leech {
				// Free up memory by clearing the leech blockstore at the end of each run.
				// Note that although we create a new blockstore for the leech at the
				// start of the run, explicitly cleaning up the blockstore from the
				// previous run allows it to be GCed.
				if err := utils.ClearBlockstore(ctx, bstore); err != nil {
					return fmt.Errorf("Error clearing blockstore: %w", err)
				}
			}
		}
		if nodetp == utils.Seed {
			// Free up memory by clearing the seed blockstore at the end of each
			// set of tests over the current file size.
			if err := utils.ClearBlockstore(ctx, bstore); err != nil {
				return fmt.Errorf("Error clearing blockstore: %w", err)
			}
		}
	}

	/// --- Ending the test

	return nil
}

func setupSeed(ctx context.Context, node *utils.Node, fileSize int) (cid.Cid, error) {
	tmpFile := utils.RandReader(fileSize)
	ipldNode, err := node.Add(ctx, tmpFile)
	if err != nil {
		return cid.Cid{}, err
	}

	return ipldNode.Cid(), nil
}

func getRootCidSubtree(id int) *sync.Subtree {
	return &sync.Subtree{
		GroupKey:    fmt.Sprintf("root-cid-%d", id),
		PayloadType: reflect.TypeOf(&cid.Cid{}),
		KeyFunc: func(val interface{}) string {
			return val.(*cid.Cid).String()
		},
	}
}

func emitMetrics(runenv *runtime.RunEnv, bsnode *utils.Node, runNum int, seq int64,
	latency time.Duration, bandwidthMB int, fileSize int, nodetp utils.NodeType, tpindex int, timeToFetch time.Duration) error {

	stats, err := bsnode.Bitswap.Stat()
	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	latencyMS := latency.Milliseconds()
	id := fmt.Sprintf("latencyMS:%d/bandwidthMB:%d/run:%d/seq:%d/file-size:%d/%s:%d", latencyMS, bandwidthMB, runNum, seq, fileSize, nodetp, tpindex)
	if nodetp == utils.Leech {
		runenv.RecordMetric(utils.MetricTimeToFetch(id), float64(timeToFetch))
	}
	runenv.RecordMetric(utils.MetricMsgsRcvd(id), float64(stats.MessagesReceived))
	runenv.RecordMetric(utils.MetricDataSent(id), float64(stats.DataSent))
	runenv.RecordMetric(utils.MetricDataRcvd(id), float64(stats.DataReceived))
	runenv.RecordMetric(utils.MetricDupDataRcvd(id), float64(stats.DupDataReceived))
	runenv.RecordMetric(utils.MetricBlksSent(id), float64(stats.BlocksSent))
	runenv.RecordMetric(utils.MetricBlksRcvd(id), float64(stats.BlocksReceived))
	runenv.RecordMetric(utils.MetricDupBlksRcvd(id), float64(stats.DupBlksReceived))

	return nil
}

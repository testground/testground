package test

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

type nodeType int

const (
	// Seeds data
	Seed nodeType = iota
	// Fetches data from seeds
	Leech
	// Doesn't seed or fetch data
	Passive
)

func (nt nodeType) String() string {
	return [...]string{"Seed", "Leech", "Passive"}[nt]
}

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
	fileSizes, err := parseFileSizes(runenv)
	if err != nil {
		return err
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(runenv)

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

	// Set up network (with traffic shaping)
	if err := utils.SetupNetwork(ctx, runenv, watcher, writer); err != nil {
		return fmt.Errorf("Failed to set up network %w", err)
	}

	// Create libp2p node
	h, err := libp2p.New(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	// Get sequence number of this host
	seq, err := writer.Write(sync.PeerSubtree, host.InfoFromHost(h))
	if err != nil {
		return err
	}

	runenv.Message("I am %s with addrs: %v", node.Host.ID(), node.Host.Addrs())

	/// --- Warm up

	// Note: seq starts at 1 (not 0)
	isLeech := seq <= int64(leechCount)
	isSeed := seq > int64(leechCount+passiveCount)
	if isLeech {
		runenv.Message("I am a leech")
	} else if isSeed {
		runenv.Message("I am a seed")
	} else {
		runenv.Message("I am a passive node (neither leech nor seed)")
	}

	var rootCid cid.Cid
	if isSeed {
		// Generate a file of the given size and add it to the datastore
		rootCid, err := setupSeed(ctx, node, fileSize)
		if err != nil {
			return err
		}

		// Inform other nodes of the root CID
		if _, err = writer.Write(RootCidSubtree, &rootCid); err != nil {
			return fmt.Errorf("Failed to get Redis Sync RootCidSubtree %w", err)
		}
	} else if isLeech {
		// Get the root CID from a seed
		rootCidCh := make(chan *cid.Cid, 1)
		cancelRootCidSub, err := watcher.Subscribe(RootCidSubtree, rootCidCh)
		if err != nil {
			return fmt.Errorf("Failed to subscribe to RootCidSubtree %w", err)
		}

		// Note: only need to get the root CID from one seed - it should be the
		// same on all seeds (seed data is generated from repeatable random
		// sequence)
		select {
		case rootCidPtr := <-rootCidCh:
			cancelRootCidSub()
			rootCid = *rootCidPtr
		case <-time.After(timeout):
			cancelRootCidSub()
			return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
		}
	}

	// Get addresses of all peers
	peerCh := make(chan *peer.AddrInfo)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
	addrInfos, err := utils.AddrInfosFromChan(peerCh, runenv.TestInstanceCount, timeout)
	if err != nil {
		cancelSub()
		return err
	}
	cancelSub()

	/// --- Warm up

	runenv.Message("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Note: seq starts at 1 (not 0)
	var nodetp nodeType
	switch {
	case seq <= int64(leechCount):
		nodetp = Leech
		runenv.Message("I am a leech")
	case seq > int64(leechCount+passiveCount):
		nodetp = Seed
		runenv.Message("I am a seed")
	default:
		nodetp = Passive
		runenv.Message("I am a passive node (neither leech nor seed)")
	}

	// Use the same blockstore on all runs for the seed node
	var bstore blockstore.Blockstore
	if nodetp == Seed {
		bstore, err = utils.CreateBlockstore(ctx, bstoreDelay)
		if err != nil {
			return err
		}
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) {
		utils.SignalAndWaitForAll(ctx, runenv.TestInstanceCount, state, watcher, writer)
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
			signalAndWaitForAll("start-run-" + runId)

			runenv.Message("Starting run %d / %d (%d bytes)", runNum, runCount, fileSize)
			var bsnode *utils.Node
			rootCidSubtree := getRootCidSubtree(sizeIndex)

			switch nodetp {
			case Seed:
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
						runenv.Message("Generating seed data of %d bytes", fileSize)
						start = time.Now()
					}

					rootCid, err := setupSeed(ctx, bsnode, fileSize)
					if err != nil {
						return fmt.Errorf("Failed to set up seed: %w", err)
					}

					if genSeedSerial {
						runenv.Message("Done generating seed data of %d bytes (%s)", fileSize, time.Since(start))

						// Signal we've completed generating the seed data
						_, err = writer.SignalEntry(seedGenerated)
						if err != nil {
							return fmt.Errorf("Failed to signal seed generated: %w", err)
						}
					}

					// Inform other nodes of the root CID
					if _, err = writer.Write(rootCidSubtree, &rootCid); err != nil {
						return fmt.Errorf("Failed to get Redis Sync rootCidSubtree %w", err)
					}
				}
			case Leech:
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
					cancelRootCidSub, err := watcher.Subscribe(rootCidSubtree, rootCidCh)
					if err != nil {
						return fmt.Errorf("Failed to subscribe to rootCidSubtree %w", err)
					}

					// Note: only need to get the root CID from one seed - it should be the
					// same on all seeds (seed data is generated from repeatable random
					// sequence)
					select {
					case rootCidPtr := <-rootCidCh:
						cancelRootCidSub()
						rootCid = *rootCidPtr
					case <-time.After(timeout):
						cancelRootCidSub()
						return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
					}
				}
			}

			// Wait for all nodes to be ready to dial
			signalAndWaitForAll("ready-to-connect-" + runId)

			// Dial all peers
			dialed, err := utils.DialOtherPeers(ctx, h, addrInfos)
			if err != nil {
				return err
			}
			runenv.Message("Dialed %d other nodes", len(dialed))

			// Wait for all nodes to be connected
			signalAndWaitForAll("connect-complete-" + runId)

			/// --- Start test

			var timeToFetch time.Duration
			if nodetp == Leech {
				// Stagger the start of the first request from each leech
				// Note: seq starts from 1 (not 0)
				startDelay := time.Duration(seq-1) * requestStagger
				time.Sleep(startDelay)

				runenv.Message("Leech fetching data after %s delay", startDelay)
				start := time.Now()
				err := bsnode.FetchGraph(ctx, rootCid)
				timeToFetch = time.Since(start)
				if err != nil {
					return fmt.Errorf("Error fetching data through Bitswap: %w", err)
				}
				runenv.Message("Leech fetch complete (%s)", timeToFetch)
			}

			// Wait for all leeches to have downloaded the data from seeds
			signalAndWaitForAll("transfer-complete-" + runId)

			/// --- Report stats
			err = emitMetrics(runenv, bsnode, runNum, seq, fileSize, nodetp, timeToFetch)
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

			if nodetp == Leech {
				// Free up memory by clearing the leech blockstore at the end of each run.
				// Note that although we create a new blockstore for the leech at the
				// start of the run, explicitly cleaning up the blockstore from the
				// previous run allows it to be GCed.
				if err := utils.ClearBlockstore(ctx, bstore); err != nil {
					runenv.Abort(fmt.Errorf("Error clearing blockstore: %w", err))
					return
				}
			}
		}
		if nodetp == Seed {
			// Free up memory by clearing the seed blockstore at the end of each
			// set of tests over the current file size.
			if err := utils.ClearBlockstore(ctx, bstore); err != nil {
				runenv.Abort(fmt.Errorf("Error clearing blockstore: %w", err))
				return
			}
		}
	}

	/// --- Ending the test

	return nil
}

func parseFileSizes(runenv *runtime.RunEnv) ([]int, error) {
	var fileSizes []int
	sizeStrs := strings.Split(runenv.StringParam("file_size"), ",")
	for _, sizeStr := range sizeStrs {
		size, err := strconv.ParseInt(sizeStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Could not convert file size '%s' to integer(s)", sizeStrs)
		}
		fileSizes = append(fileSizes, int(size))
	}
	return fileSizes, nil
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

func emitMetrics(runenv *runtime.RunEnv, bsnode *utils.Node, runNum int, seq int64, fileSize int, nodetp nodeType, timeToFetch time.Duration) error {
	stats, err := bsnode.Bitswap.Stat()
	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	id := fmt.Sprintf("run:%d/seq:%d/file-size:%d/%s", runNum, seq, fileSize, nodetp)
	if nodetp == Leech {
		runenv.EmitMetric(utils.MetricTimeToFetch(id), float64(timeToFetch))
	}
	runenv.EmitMetric(utils.MetricMsgsRcvd(id), float64(stats.MessagesReceived))
	runenv.EmitMetric(utils.MetricDataSent(id), float64(stats.DataSent))
	runenv.EmitMetric(utils.MetricDataRcvd(id), float64(stats.DataReceived))
	runenv.EmitMetric(utils.MetricDupDataRcvd(id), float64(stats.DupDataReceived))
	runenv.EmitMetric(utils.MetricBlksSent(id), float64(stats.BlocksSent))
	runenv.EmitMetric(utils.MetricBlksRcvd(id), float64(stats.BlocksReceived))
	runenv.EmitMetric(utils.MetricDupBlksRcvd(id), float64(stats.DupBlksReceived))
}

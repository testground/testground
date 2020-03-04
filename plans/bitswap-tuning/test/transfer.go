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

	ipld "github.com/ipfs/go-ipld-format"
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
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")
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
	runenv.Message("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Get sequence number of this host
	seq, err := writer.Write(ctx, sync.PeerSubtree, host.InfoFromHost(h))
	if err != nil {
		return err
	}
	grpseq, nodetp, tpindex, err := parseType(ctx, runenv, writer, h, seq)
	if err != nil {
		return err
	}

	var seedIndex int64
	if nodetp == utils.Seed {
		if runenv.TestGroupID == "" {
			// If we're not running in group mode, calculate the seed index as
			// the sequence number minus the other types of node (leech / passive).
			// Note: sequence number starts from 1 (not 0)
			seedIndex = seq - int64(leechCount+passiveCount) - 1
		} else {
			// If we are in group mode, signal other seed nodes to work out the
			// seed index
			seedSeq, err := getNodeSetSeq(ctx, writer, h, "seeds")
			if err != nil {
				return err
			}
			// Sequence number starts from 1 (not 0)
			seedIndex = seedSeq - 1
		}
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
		// genSeedSerial := seedCount > 2 || fileSize*seedCount > parallelGenMax
		genSeedSerial := true

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
						if seedIndex > 0 {
							// Wait for the seeds with an index lower than this one
							// to generate their seed data
							doneCh := watcher.Barrier(ctx, seedGenerated, int64(seedIndex))
							if err = <-doneCh; err != nil {
								return err
							}
						}

						// Generate a file of the given size and add it to the datastore
						start = time.Now()
					}
					runenv.Message("Generating seed data of %d bytes", fileSize)

					rootCid, err := setupSeed(ctx, runenv, bsnode, fileSize, int(seedIndex))
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
			err = emitMetrics(runenv, bsnode, runNum, seq, grpseq, latency, bandwidthMB, fileSize, nodetp, tpindex, timeToFetch)
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

func parseType(ctx context.Context, runenv *runtime.RunEnv, writer *sync.Writer, h host.Host, seq int64) (int64, utils.NodeType, int, error) {
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")

	grpCountOverride := false
	if runenv.TestGroupID != "" {
		grpLchLabel := runenv.TestGroupID + "_leech_count"
		if runenv.IsParamSet(grpLchLabel) {
			leechCount = runenv.IntParam(grpLchLabel)
			grpCountOverride = true
		}
		grpPsvLabel := runenv.TestGroupID + "_passive_count"
		if runenv.IsParamSet(grpPsvLabel) {
			passiveCount = runenv.IntParam(grpPsvLabel)
			grpCountOverride = true
		}
	}

	var nodetp utils.NodeType
	var tpindex int
	grpseq := seq
	seqstr := fmt.Sprintf("- seq %d / %d", seq, runenv.TestInstanceCount)
	grpPrefix := ""
	if grpCountOverride {
		grpPrefix = runenv.TestGroupID + " "

		var err error
		grpseq, err = getNodeSetSeq(ctx, writer, h, runenv.TestGroupID)
		if err != nil {
			return grpseq, nodetp, tpindex, err
		}

		seqstr = fmt.Sprintf("%s (%d / %d of %s)", seqstr, grpseq, runenv.TestGroupInstanceCount, runenv.TestGroupID)
	}

	// Note: seq starts at 1 (not 0)
	switch {
	case grpseq <= int64(leechCount):
		nodetp = utils.Leech
		tpindex = int(grpseq) - 1
	case grpseq > int64(leechCount+passiveCount):
		nodetp = utils.Seed
		tpindex = int(grpseq) - 1 - (leechCount + passiveCount)
	default:
		nodetp = utils.Passive
		tpindex = int(grpseq) - 1 - leechCount
	}

	runenv.Message("I am %s %d %s", grpPrefix+nodetp.String(), tpindex, seqstr)

	return grpseq, nodetp, tpindex, nil
}

func getNodeSetSeq(ctx context.Context, writer *sync.Writer, h host.Host, setID string) (int64, error) {
	subtree := &sync.Subtree{
		GroupKey:    "nodes" + setID,
		PayloadType: reflect.TypeOf(&peer.AddrInfo{}),
		KeyFunc: func(val interface{}) string {
			return val.(*peer.AddrInfo).ID.Pretty()
		},
	}

	return writer.Write(ctx, subtree, host.InfoFromHost(h))
}

func setupSeed(ctx context.Context, runenv *runtime.RunEnv, node *utils.Node, fileSize int, seedIndex int) (cid.Cid, error) {
	tmpFile := utils.RandReader(fileSize)
	ipldNode, err := node.Add(ctx, tmpFile)
	if err != nil {
		return cid.Cid{}, err
	}

	if !runenv.IsParamSet("seed_fraction") {
		return ipldNode.Cid(), nil
	}
	seedFrac := runenv.StringParam("seed_fraction")
	if seedFrac == "" {
		return ipldNode.Cid(), nil
	}

	parts := strings.Split(seedFrac, "/")
	if len(parts) != 2 {
		return cid.Cid{}, fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}
	numerator, nerr := strconv.ParseInt(parts[0], 10, 64)
	denominator, derr := strconv.ParseInt(parts[1], 10, 64)
	if nerr != nil || derr != nil {
		return cid.Cid{}, fmt.Errorf("Invalid seed fraction %s", seedFrac)
	}

	nodes, err := getLeafNodes(ctx, ipldNode, node.Dserv)
	if err != nil {
		return cid.Cid{}, err
	}
	var del []cid.Cid
	for i := 0; i < len(nodes); i++ {
		idx := i + seedIndex
		if idx%int(denominator) >= int(numerator) {
			del = append(del, nodes[i].Cid())
		}
	}
	if err := node.Dserv.RemoveMany(ctx, del); err != nil {
		return cid.Cid{}, err
	}

	runenv.Message("Retained %d / %d of blocks from seed, removed %d / %d blocks", numerator, denominator, len(del), len(nodes))
	return ipldNode.Cid(), nil
}

func getLeafNodes(ctx context.Context, node ipld.Node, dserv ipld.DAGService) ([]ipld.Node, error) {
	if len(node.Links()) == 0 {
		return []ipld.Node{node}, nil
	}

	var leaves []ipld.Node
	for _, l := range node.Links() {
		child, err := l.GetNode(ctx, dserv)
		if err != nil {
			return nil, err
		}
		childLeaves, err := getLeafNodes(ctx, child, dserv)
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, childLeaves...)
	}

	return leaves, nil
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

func emitMetrics(runenv *runtime.RunEnv, bsnode *utils.Node, runNum int, seq int64, grpseq int64,
	latency time.Duration, bandwidthMB int, fileSize int, nodetp utils.NodeType, tpindex int, timeToFetch time.Duration) error {

	stats, err := bsnode.Bitswap.Stat()
	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	latencyMS := latency.Milliseconds()
	id := fmt.Sprintf("latencyMS:%d/bandwidthMB:%d/run:%d/seq:%d/groupName:%s/groupSeq:%d/fileSize:%d/nodeType:%s/nodeTypeIndex:%d",
		latencyMS, bandwidthMB, runNum, seq, runenv.TestGroupID, grpseq, fileSize, nodetp, tpindex)
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

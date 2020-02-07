package test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/testground/plans/bitswap-tuning/utils"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// NOTE: To run use:
// go build . && TESTGROUND_BASEDIR=`pwd` ./testground run single data-exchange/transfer --builder=docker:go --runner="local:docker" --dep="github.com/ipfs/go-bitswap=master"

var RootCidSubtree = &sync.Subtree{
	GroupKey:    "root-cid",
	PayloadType: reflect.TypeOf(&cid.Cid{}),
	KeyFunc: func(val interface{}) string {
		return val.(*cid.Cid).String()
	},
}

// Transfer data from S seeds to L leeches
func Transfer(runenv *runtime.RunEnv) error {
	// Test Parameters
	timeout := time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")
	requestStagger := time.Duration(runenv.IntParam("request_stagger")) * time.Millisecond
	fileSize := runenv.IntParam("file_size")

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
	node, err := utils.CreateNode(ctx, runenv)
	if err != nil {
		return err
	}
	defer node.Close()

	// Get sequence number of this host
	seq, err := writer.Write(sync.PeerSubtree, host.InfoFromHost(node.Host))
	if err != nil {
		return err
	}

	runenv.RecordMessage("I am %s with addrs: %v", node.Host.ID(), node.Host.Addrs())

	/// --- Warm up

	// Note: seq starts at 1 (not 0)
	isLeech := seq <= int64(leechCount)
	isSeed := seq > int64(leechCount+passiveCount)
	if isLeech {
		runenv.RecordMessage("I am a leech")
	} else if isSeed {
		runenv.RecordMessage("I am a seed")
	} else {
		runenv.RecordMessage("I am a passive node (neither leech nor seed)")
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
			_ = cancelRootCidSub()
			rootCid = *rootCidPtr
		case <-time.After(timeout):
			_ = cancelRootCidSub()
			return fmt.Errorf("no root cid in %d seconds", timeout/time.Second)
		}
	}

	// Get addresses of all peers
	peerCh := make(chan *peer.AddrInfo)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
	if err != nil {
		return err
	}
	addrInfos, err := utils.AddrInfosFromChan(peerCh, runenv.TestInstanceCount, timeout)
	if err != nil {
		_ = cancelSub()
		return err
	}
	_ = cancelSub()

	// Dial all peers
	dialed, err := utils.DialOtherPeers(ctx, node.Host, addrInfos)
	if err != nil {
		return err
	}
	runenv.RecordMessage("Dialed %d other nodes", len(dialed))

	err = utils.SignalAndWaitForAll(ctx, runenv.TestInstanceCount, "ready", watcher, writer)
	if err != nil {
		return err
	}

	/// --- Act I

	start := time.Now()

	if isLeech {
		// Stagger the start of the first request from each leech
		// Note: seq starts from 1 (not 0)
		startDelay := time.Duration(seq-1) * requestStagger
		time.Sleep(startDelay)

		runenv.RecordMessage("Leech fetching after %s delay", startDelay)
		err = node.FetchGraph(ctx, rootCid)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Leech fetch complete")
	} else {
		runenv.RecordMessage("Seed ready")
	}

	err = utils.SignalAndWaitForAll(ctx, runenv.TestInstanceCount, "transfer-complete", watcher, writer)
	if err != nil {
		return err
	}

	stats, err := node.Bitswap.Stat()
	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	if isLeech {
		runenv.RecordMetric(utils.MetricTimeToFetch, float64(time.Since(start).Nanoseconds()))
	}
	runenv.RecordMetric(utils.MetricMsgsRcvd, float64(stats.MessagesReceived))
	runenv.RecordMetric(utils.MetricDataSent, float64(stats.DataSent))
	runenv.RecordMetric(utils.MetricDataRcvd, float64(stats.DataReceived))
	runenv.RecordMetric(utils.MetricDupDataRcvd, float64(stats.DupDataReceived))
	runenv.RecordMetric(utils.MetricBlksSent, float64(stats.BlocksSent))
	runenv.RecordMetric(utils.MetricBlksRcvd, float64(stats.BlocksReceived))
	runenv.RecordMetric(utils.MetricDupBlksRcvd, float64(stats.DupBlksReceived))

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

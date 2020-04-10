package test

import (
	"context"
	"fmt"
	"github.com/ipfs/testground/plans/dht/utils"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

func FindPeers(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	ri, err := Base(ctx, runenv, commonOpts)
	if err != nil {
		return err
	}

	if err := TestFindPeers(ctx, ri); err != nil {
		return err
	}
	Teardown(ctx, ri.RunInfo)

	return nil
}

func TestFindPeers(ctx context.Context, ri *DHTRunInfo) error {
	runenv := ri.RunEnv

	nFindPeers := runenv.IntParam("n_find_peers")

	if nFindPeers > runenv.TestInstanceCount {
		return fmt.Errorf("NFindPeers greater than the number of test instances")
	}

	node := ri.Node
	peers := ri.Others

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "peer-records", ri.RunInfo)

	// Ok, we're _finally_ ready.
	// TODO: Dump routing table stats. We should dump:
	//
	// * How full our "closest" bucket is. That is, look at the "all peers"
	//   list, find the BucketSize closest peers, and determine the % of those
	//   peers to which we're connected. It should be close to 100%.
	// * How many peers we're actually connected to?
	// * How many of our connected peers are actually useful to us?

	// Perform FIND_PEER N times.

	if err := stager.Begin(); err != nil {
		return err
	}

	found := 0
	for p, info := range peers {
		if found >= nFindPeers {
			break
		}
		if len(node.host.Peerstore().Addrs(p)) > 0 {
			// Skip peer's we've already found (even if we've
			// disconnected for some reason).
			continue
		}

		if info.Properties.Undialable || node.info.Addrs.ID == info.Addrs.ID {
			continue
		}

		runenv.RecordMessage("start find peer number %d", found+1)

		ectx, cancel := context.WithCancel(ctx)
		ectx = TraceQuery(ectx, runenv, node, p.Pretty(), "peer-records")

		t := time.Now()

		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		_, err := node.dht.FindPeer(ectx, p)
		cancel()
		if err != nil {
			runenv.RecordMessage("find peer failed: peer %s : %s", p, err)
			runenv.RecordMetric(&runtime.MetricDefinition{
				Name:           fmt.Sprintf("time-to-failed-peer-%d", found),
				Unit:           "ns",
				ImprovementDir: -1,
			}, float64(time.Since(t).Nanoseconds()))
		} else {
			runenv.RecordMetric(&runtime.MetricDefinition{
				Name:           fmt.Sprintf("time-to-peer-%d", found),
				Unit:           "ns",
				ImprovementDir: -1,
			}, float64(time.Since(t).Nanoseconds()))
		}

		found++
	}

	if err := stager.End(); err != nil {
		return err
	}

	return nil
}

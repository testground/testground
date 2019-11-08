package test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/libp2p/go-libp2p-core/peer"
)

// FindPeers is the Find Peers Test Case
func FindPeers(runenv *runtime.RunEnv) {
	// Test Parameters
	var (
		timeout     = time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second
		randomWalk  = runenv.BooleanParamD("random_walk", false)
		nFindPeers  = runenv.IntParamD("n_find_peers", 1)
		bucketSize  = runenv.IntParamD("bucket_size", 20)
		autoRefresh = runenv.BooleanParamD("auto_refresh", true)
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	node, dht, toDial, err := SetUp(ctx, runenv, timeout, randomWalk, bucketSize, autoRefresh)
	if err != nil {
		runenv.Abort(err)
		return
	}

	/// --- Act I

	for i := 0; i < nFindPeers; i++ {
		var peerToFind peer.AddrInfo

		// This search is suboptimal -> TODO check if go-libp2p has funcs or maps to help make this faster
	SelectPeer:
		for _, anotherPeer := range toDial {
			for _, connectedPeer := range node.Peerstore().PeersWithAddrs() {
				apID, _ := anotherPeer.ID.MarshalBinary()
				cpID, _ := connectedPeer.MarshalBinary()
				if bytes.Compare(apID, cpID) == 0 {
					continue // already dialed to this one, next
				}
				// found a peer from list that we are not yet connected
				peerToFind = anotherPeer
				break SelectPeer
			}
		}
		// Find Peer dance
		t := time.Now()
		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed
		if _, err := dht.FindPeer(ctx, peerToFind.ID); err != nil {
			runenv.Message("FindPeer failed %w", err)
			return
		}

		runenv.EmitMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))
	}

	/// --- Ending the test

	runenv.OK()
}

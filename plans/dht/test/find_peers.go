package test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/testground/plans/dht/utils"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// NOTE: Needs to run with latest kad-dht. Use:
//  go build . && TESTGROUND_BASEDIR=`pwd` ./testground run dht/find-peers --builder=docker:go --runner="local:docker" --dep="github.com/libp2p/go-libp2p-kad-dht=master" --build-cfg bypass_cache=true

// FindPeers is the Find Peers Test Case
func FindPeers(runenv *runtime.RunEnv) {
	// Test Parameters
	var (
		timeout    = time.Duration(runenv.IntParamD("timeout_secs", 60)) * time.Second
		randomWalk = runenv.BooleanParamD("random_walk", false)
		nFindPeers = runenv.IntParamD("n_find_peers", 1)
	)

	/// --- Set upA

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	node, dht, err := utils.CreateDhtNode(ctx, runenv)
	if err != nil {
		runenv.Abort(err)
		return
	}

	myNodeID := node.ID()
	runenv.Message("I am %s with addrs: %v", myNodeID, node.Addrs())

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	/// --- Tear down

	defer func() {
		// Set a state barrier.
		end := sync.State("end")
		doneCh := watcher.Barrier(ctx, end, int64(runenv.TestInstanceCount))

		// Signal we're done on the end state.
		_, err = writer.SignalEntry(end)
		if err != nil {
			runenv.Abort(err)
			return
		}

		// Wait until all others have signalled.
		if err := <-doneCh; err != nil {
			runenv.Abort(err)
			return
		}
	}()

	/// --- Warm up

	if _, err = writer.Write(sync.PeerSubtree, host.InfoFromHost(node)); err != nil {
		runenv.Abort(fmt.Errorf("Failed to get Redis Sync PeerSubtree %w", err))
		return
	}

	// TODO: Revisit this - This assumed that it is ok to put in memory every single peer.AddrInfo that participates in this test
	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, sync.TypedChan(peerCh))
	defer cancelSub()

	var toDial []peer.AddrInfo
	// Grab list of other peers that are available for this Run
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:
			id1, _ := ai.ID.MarshalBinary()
			id2, _ := myNodeID.MarshalBinary()
			if bytes.Compare(id1, id2) >= 0 {
				// skip over dialing ourselves, and prevent TCP simultaneous
				// connect (known to fail) by only dialing peers whose peer ID
				// is smaller than ours.
				continue
			}
			toDial = append(toDial, *ai)

		case <-time.After(timeout):
			runenv.Abort(fmt.Errorf("no new peers in %d seconds", timeout/time.Second))
			return
		}
	}

	// Dial to all the other peers
	for _, ai := range toDial {
		err = node.Connect(ctx, ai)
		if err != nil {
			runenv.Abort(fmt.Errorf("Error while dialing peer %v: %w", ai.Addrs, err))
			return
		}
	}

	runenv.Message("Dialed all my buds")

	// Check if `random-walk` is enabled, if yes, run it 5 times
	for i := 0; randomWalk && i < 5; i++ {
		err = dht.Bootstrap(ctx)
		if err != nil {
			runenv.Abort(fmt.Errorf("Could not run a random-walk: %w", err))
			return
		}
	}

Loop:
	for {
		select {
		case <-time.After(200 * time.Millisecond):
			if dht.RoutingTable().Size() > 0 {
				break Loop
			}
		case <-ctx.Done():
			runenv.Abort(fmt.Errorf("got no peers in routing table"))
			return
		}
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

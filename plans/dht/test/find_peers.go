package test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	utils "github.com/ipfs/testground/plans/dht/utils"
	host "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

// FindPeers is the Find Peers Test Case
func FindPeers(runenv *runtime.RunEnv) {
	// Test Parameters
	var (
		timeout = time.Duration(runenv.IntParamD("timeout_secs", 30)) * time.Second
	)

	/// --- Warm up
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

	if _, err = writer.Write(sync.PeerSubtree, host.InfoFromHost(node)); err != nil {
		runenv.Abort(err)
		return
	}
	defer writer.Close()

	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, sync.TypedChan(peerCh))
	defer cancelSub()

	var toDial []*peer.AddrInfo
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
			toDial = append(toDial, ai)

		case <-time.After(timeout):
			// TODO need a way to fail a distributed test immediately. No point
			// making it run elsewhere beyond this point.
			runenv.Abort(fmt.Errorf("no new peers in %d seconds", timeout/time.Second))
			return
		}
	}

	// Dial to all the other peers
	for _, ai := range toDial {
		err = node.Connect(ctx, *ai)
		if err != nil {
			runenv.Abort(fmt.Errorf("error while dialing peer %v: %w", ai.Addrs, err))
			return
		}
	}

	// TODO: Check if `random-walk` is enabled, if yes, run it 5 times

	/// --- Act I

	for i, id := range node.Peerstore().PeersWithAddrs() {
		t := time.Now()
		// TODO: Instrument libp2p dht to get:
		// - Number of peers dialed
		// - Number of dials along the way that failed

		// TODO call FindPeer `n-find-peers` times, each time a different peer
		if _, err := dht.FindPeer(ctx, id); err != nil {
			runenv.Abort(err)
			return
		}

		runenv.EmitMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))
	}

	end := sync.State("end")

	/// --- Ending the test

	// Set a state barrier.
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

	runenv.OK()
}

package test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
)

// SetUp sets up the elements necessary for the test cases
func SetUp(ctx context.Context, runenv *runtime.RunEnv, timeout time.Duration, randomWalk bool, bucketSize int, autoRefresh bool) (host.Host, *kaddht.IpfsDHT, []peer.AddrInfo, error) {
	/// --- Set up

	node, dht, err := CreateDhtNode(ctx, runenv, bucketSize, autoRefresh)
	if err != nil {
		return nil, nil, nil, err
	}

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

	myNodeID := node.ID()
	runenv.Message("I am %s with addrs: %v", myNodeID, node.Addrs())

	if _, err = writer.Write(sync.PeerSubtree, host.InfoFromHost(node)); err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to get Redis Sync PeerSubtree %w", err)
	}

	// TODO: Revisit this - This assumed that it is ok to put in memory every single peer.AddrInfo that participates in this test
	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
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
			return nil, nil, nil, fmt.Errorf("no new peers in %d seconds", timeout/time.Second)
		}
	}

	// Dial to all the other peers
	for _, ai := range toDial {
		err = node.Connect(ctx, ai)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("Error while dialing peer %v: %w", ai.Addrs, err)
		}
	}

	runenv.Message("Dialed all my buds")

	// Check if `random-walk` is enabled, if yes, run it 5 times
	for i := 0; randomWalk && i < 5; i++ {
		err = dht.Bootstrap(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("Could not run a random-walk: %w", err)
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
			return nil, nil, nil, fmt.Errorf("got no peers in routing table")
		}
	}

	return node, dht, toDial, nil
}

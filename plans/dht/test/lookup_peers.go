package test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"

	"github.com/ipfs/go-datastore"
)

func LookupPeers(runenv *runtime.RunEnv) {
	timeout := func() time.Duration {
		if t, ok := runenv.IntParam("timeout_secs"); !ok {
			return 30 * time.Second
		} else {
			return time.Duration(t) * time.Second
		}
	}()

	// TODO Make parameter getting nicer by providing typed accessors in the
	// RunEnv (à la urfave/cli), that accept a second argument (default value).
	bucketSize := func() int {
		if t, ok := runenv.IntParam("bucket_size"); !ok {
			return 20
		} else {
			return t
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// moot assignment to pacify the compiler. We need to merge the configurable
	// bucket size param upstream.
	_ = bucketSize

	h, err := libp2p.New(context.Background())
	if err != nil {
		runenv.Abort(err)
		return
	}

	runenv.Message("I am %s with addrs: %v", h.ID(), h.Addrs())

	dht, err := dht.New(context.Background(), h, dhtopts.Datastore(datastore.NewMapDatastore()))
	if err != nil {
		runenv.Abort(err)
		return
	}

	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()

	if _, err = writer.Write(sync.PeerSubtree, host.InfoFromHost(h)); err != nil {
		runenv.Abort(err)
		return
	}
	defer writer.Close()

	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, sync.TypedChan(peerCh))
	defer cancelSub()

	var toDial []*peer.AddrInfo
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:
			id1, _ := ai.ID.MarshalBinary()
			id2, _ := h.ID().MarshalBinary()
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

	for _, ai := range toDial {
		err = h.Connect(ctx, *ai)
		if err != nil {
			runenv.Abort(fmt.Errorf("error while dialing peer %v: %w", ai.Addrs, err))
			return
		}
	}

	for i, id := range h.Peerstore().PeersWithAddrs() {
		t := time.Now()
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

package main

import (
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
	h, err := libp2p.New(context.Background())
	if err != nil {
		panic(err)
	}

	dht, err := dht.New(context.Background(), h, dhtopts.Datastore(datastore.NewMapDatastore()))
	if err != nil {
		panic(err)
	}

	redis, err := sync.RedisClient()
	if err != nil {
		panic(err)
	}

	watcher, writer := sync.MustWatcherWriter(redis, runenv)
	defer watcher.Close()

	if err = writer.Write(sync.PeerSubtree, host.InfoFromHost(h)); err != nil {
		panic(err)
	}
	defer writer.Close()

	peerCh := make(chan *peer.AddrInfo, 16)
	cancel, err := watcher.Subscribe(sync.PeerSubtree, sync.TypedChan(peerCh))
	defer cancel()

	var events int
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:
			events++
			if ai.ID == h.ID() {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := h.Connect(ctx, *ai)
			if err != nil {
				panic(err)
			}
			cancel()

		case <-time.After(10 * time.Second):
			// TODO need a way to fail a distributed test immediately. No point
			// making it run.
			panic("no new peers in 10 seconds")
		}
	}

	for i, id := range h.Peerstore().PeersWithAddrs() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t := time.Now()
		if _, err := dht.FindPeer(ctx, id); err != nil {
			panic(err)
		}

		runtime.EmitMetric(runtime.NewContextWithRunEnv(context.Background()), &runtime.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))

		cancel()
	}

	defer cancel()
}

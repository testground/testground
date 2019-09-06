package dht

import (
	"context"
	"fmt"
	"io/ioutil"

	levelds "github.com/ipfs/go-ds-leveldb"
	"github.com/ipfs/testground/api"
	"github.com/ipfs/testground/sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"

	"time"
)

type LookupPeersTC struct {
	Instance       int
	Count          int
	BucketSize     int
	EventsReceived int
}

//var _ dht.DHTTestCase = (*LookupPeersTC)(nil)

func (tc *LookupPeersTC) Name() string {
	return fmt.Sprintf("lookup_peers-%dpeers-%dsize", tc.Count, tc.BucketSize)
}

func (tc *LookupPeersTC) Execute() {
	dir, err := ioutil.TempDir("", "dht")
	if err != nil {
		panic(err)
	}

	ds, err := levelds.NewDatastore(dir, nil)
	if err != nil {
		panic(err)
	}

	h, err := libp2p.New(context.Background())
	if err != nil {
		panic(err)
	}

	dht, err := kaddht.New(context.Background(), h, dhtopts.Datastore(ds))
	if err != nil {
		panic(err)
	}

	runenv := api.CurrentRunEnv()
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

	for i := 0; i < tc.Count; i++ {
		select {
		case ai := <-peerCh:
			tc.EventsReceived++
			if ai.ID == h.ID() {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := h.Connect(ctx, *ai)
			if err != nil {
				panic(err)
			}
			cancel()

		case <-time.After(5 * time.Second):
			panic("no new peers in 5 seconds")
		}
	}

	for i, id := range h.Peerstore().PeersWithAddrs() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t := time.Now()
		if _, err := dht.FindPeer(ctx, id); err != nil {
			panic(err)
		}

		api.EmitMetric(api.NewContext(context.Background()), &api.MetricDefinition{
			Name:           fmt.Sprintf("time-to-find-%d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Now().Sub(t).Nanoseconds()))

		cancel()
	}

	defer cancel()

	time.Sleep(5 * time.Hour)

}

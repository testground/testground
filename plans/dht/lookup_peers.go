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

	"github.com/logrusorgru/aurora"
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

	_, err = kaddht.New(context.Background(), h, dhtopts.Datastore(ds))
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

	peerCh := make(chan *peer.AddrInfo, 16)
	cancel, err := watcher.Subscribe(sync.PeerSubtree, sync.TypedChan(peerCh))
	defer cancel()

	go func() {
		for range peerCh {
			tc.EventsReceived++
			fmt.Println(aurora.Sprintf(aurora.Green(aurora.Bold("(instance %d) received events: %d")), tc.Instance, tc.EventsReceived))
		}
	}()

	defer cancel()

	if err = writer.Write(sync.PeerSubtree, host.InfoFromHost(h)); err != nil {
		panic(err)
	}
	defer writer.Close()

	time.Sleep(5 * time.Hour)

}

package test

import (
	"context"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
)

// CreateDhtNode creates a libp2p Node and a DHT on top of it
func CreateDhtNode(ctx context.Context, runenv *runtime.RunEnv, bucketSize int, autoRefresh bool) (host.Host, *kaddht.IpfsDHT, error) {
	node, err := libp2p.New(ctx)
	if err != nil {
		return nil, nil, err
	}

	dhtOptions := []dhtopts.Option{
		dhtopts.Datastore(datastore.NewMapDatastore()),
		dhtopts.BucketSize(bucketSize),
	}

	if !autoRefresh {
		dhtOptions = append(dhtOptions, dhtopts.DisableAutoRefresh())
	}

	dht, err := kaddht.New(ctx, node, dhtOptions...)
	if err != nil {
		return nil, nil, err
	}
	return node, dht, nil
}

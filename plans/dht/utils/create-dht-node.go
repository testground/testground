package utils

import (
	"context"

	"github.com/ipfs/testground/sdk/runtime"

	datastore "github.com/ipfs/go-datastore"
	libp2p "github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
)

// CreateDhtNode creates a libp2p Node and a DHT on top of it
func CreateDhtNode(ctx context.Context, runenv *runtime.RunEnv) (host.Host, *kaddht.IpfsDHT, error) {
	// Test Parameters
	var (
		bucketSize = runenv.IntParamD("bucket_size", 20)
		randomWalk = runenv.BooleanParamD("random_walk", false)
	)

	node, err := libp2p.New(ctx)
	if err != nil {
		runenv.Abort(err)
		return nil, nil, err
	}

	dhtOptions := []dhtopts.Option{
		dhtopts.Datastore(datastore.NewMapDatastore()),
		dhtopts.BucketSize(bucketSize),
	}

	if !randomWalk {
		dhtOptions = append(dhtOptions, dhtopts.DisableAutoBootstrap())
	}

	dht, err := kaddht.New(ctx, node, dhtOptions...)
	if err != nil {
		runenv.Abort(err)
		return nil, nil, err
	}
	return node, dht, nil
}

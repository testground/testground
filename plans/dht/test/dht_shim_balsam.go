// +build balsam

package test

import (
	"context"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
)

func createDHT(ctx context.Context, h host.Host, ds datastore.Batching, opts *SetupOpts, info *NodeInfo) (*kaddht.IpfsDHT, error) {
	dhtOptions := []dhtopts.Option{
		dhtopts.Protocols("/testground/kad/1.0.0"),
		dhtopts.Datastore(ds),
		dhtopts.BucketSize(opts.BucketSize),
		dhtopts.RoutingTableRefreshQueryTimeout(opts.Timeout),
	}

	if !opts.AutoRefresh {
		dhtOptions = append(dhtOptions, dhtopts.DisableAutoRefresh())
	}

	if info.Properties.Undialable && opts.ClientMode {
		dhtOptions = append(dhtOptions, dhtopts.Client(true))
	}

	dht, err := kaddht.New(ctx, h, dhtOptions...)
	if err != nil {
		return nil, err
	}
	return dht, nil
}

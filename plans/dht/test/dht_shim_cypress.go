// +build cypress

package test

import (
	"context"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
)

func createDHT(ctx context.Context, h host.Host, ds datastore.Batching, opts *SetupOpts, info *NodeInfo) (*kaddht.IpfsDHT, error) {
	dhtOptions := []kaddht.Option{
		kaddht.ProtocolPrefix("/testground"),
		kaddht.Datastore(ds),
		kaddht.BucketSize(opts.BucketSize),
		kaddht.RoutingTableRefreshQueryTimeout(opts.Timeout),
		kaddht.Concurrency(opts.Alpha),
		kaddht.Resiliency(opts.Beta),
		kaddht.DisjointPaths(opts.NDisjointPaths),
	}

	if !opts.AutoRefresh {
		dhtOptions = append(dhtOptions, kaddht.DisableAutoRefresh())
	}

	if info.Properties.Bootstrapper {
		dhtOptions = append(dhtOptions, kaddht.Mode(kaddht.ModeServer))
	} else if info.Properties.Undialable && opts.ClientMode {
		dhtOptions = append(dhtOptions, kaddht.Mode(kaddht.ModeClient))
	}

	dht, err := kaddht.New(ctx, h, dhtOptions...)
	if err != nil {
		return nil, err
	}
	return dht, nil
}

func getTaggedLibp2pOpts(opts *SetupOpts, info *NodeInfo) []libp2p.Option {
	if info.Properties.Bootstrapper {
		return []libp2p.Option{libp2p.EnableNATService(), libp2p.WithReachability(true)}
	} else {
		return []libp2p.Option{libp2p.EnableNATService()}
	}
}

// +build cypress

package test

import (
	"context"
	"github.com/ipfs/go-datastore"
	autonatsvc "github.com/libp2p/go-libp2p-autonat-svc"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
)

func createDHT(ctx context.Context, h host.Host, ds datastore.Batching, opts *SetupOpts, info *NodeInfo) (*kaddht.IpfsDHT, error) {
	dhtOptions := []kaddht.Option{
		kaddht.ProtocolPrefix("/testground"),
		kaddht.Datastore(ds),
		kaddht.BucketSize(opts.BucketSize),
		kaddht.RoutingTableRefreshQueryTimeout(opts.Timeout),
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

	if !info.Properties.Undialable {
		if _, err := autonatsvc.NewAutoNATService(ctx, h, true); err != nil {
			return nil, err
		}
	}

	dht, err := kaddht.New(ctx, h, dhtOptions...)
	if err != nil {
		return nil, err
	}
	return dht, nil
}

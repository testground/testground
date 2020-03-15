// +build balsam

package test

import (
	"context"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
)

func createDHT(ctx context.Context, h host.Host, ds datastore.Batching, opts *SetupOpts, info *NodeInfo) (*kaddht.IpfsDHT, error){
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

type DHTShim struct {
	dht *kaddht.IpfsDHT
}

func (s *DHTShim) PutValue(ctx context.Context, key string, val []byte, opts ...routing.Option) error {
	return s.dht.PutValue(ctx, key, val, opts...)
}

func (s *DHTShim) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	return s.dht.GetValue(ctx, key, opts...)
}

func (s *DHTShim) SearchValue(ctx context.Context, key string, opts ...routing.Option) (<-chan []byte, error) {
	return s.dht.SearchValue(ctx, key, opts...)
}

func (s *DHTShim) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	return s.dht.FindPeer(ctx, p)
}

func (s *DHTShim) Provide(ctx context.Context, c cid.Cid, brdcst bool) error {
	return s.dht.Provide(ctx, c, brdcst)
}

func (s *DHTShim) FindProvidersAsync(ctx context.Context, c cid.Cid, count int) <-chan peer.AddrInfo {
	return s.dht.FindProvidersAsync(ctx, c, count)
}

func (s *DHTShim) Bootstrap(ctx context.Context) error {
	return s.dht.Bootstrap(ctx)
}

func (s *DHTShim) GetPublicKey(ctx context.Context, p peer.ID) (crypto.PubKey, error) {
	return s.GetPublicKey(ctx, p)
}

var _ routing.Routing = (*DHTShim)(nil)
var _ routing.PubKeyFetcher = (*DHTShim)(nil)

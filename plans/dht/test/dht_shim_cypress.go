// +build cypress

package test

import (
	"context"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"go.uber.org/zap"
)

func createDHT(ctx context.Context, h host.Host, ds datastore.Batching, opts *SetupOpts, info *NodeInfo) (*kaddht.IpfsDHT, error) {
	dhtOptions := []kaddht.Option{
		kaddht.ProtocolPrefix("/testground"),
		kaddht.Datastore(ds),
		kaddht.BucketSize(opts.BucketSize),
		kaddht.RoutingTableRefreshQueryTimeout(opts.Timeout),
		kaddht.Concurrency(opts.Alpha),
		kaddht.Resiliency(opts.Beta),
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

func getAllProvRecordsNum() int {return 0}

var (
	sqonce   sync.Once
	sqlogger *zap.SugaredLogger
)

func specializedTraceQuery(ctx context.Context, runenv *runtime.RunEnv, node *NodeParams, target string) context.Context {
	sqonce.Do(func() {
		var err error
		_, sqlogger, err = runenv.CreateStructuredAsset("dht_lookups.out", runtime.StandardJSONConfig())
		if err != nil {
			runenv.RecordMessage("failed to initialize dht_lookups.out asset; nooping logger: %s", err)
			sqlogger = zap.NewNop().Sugar()
		}
	})

	ectx, events := kaddht.RegisterForLookupEvents(ctx)
	nodeID := node.host.ID()
	log := sqlogger.With("node", nodeID, "nodeKad", kbucket.ConvertPeerID(nodeID) , "target", target)

	go func() {
		for e := range events {
			log.Infow("lookup event", "info", e)
		}
	}()

	return ectx
}

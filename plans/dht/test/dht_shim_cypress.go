// +build cypress

package test

import (
	"context"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"github.com/libp2p/go-libp2p-xor/kademlia"
	"github.com/libp2p/go-libp2p-xor/key"
	"github.com/libp2p/go-libp2p-xor/trie"
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

func specializedTraceQuery(ctx context.Context, runenv *runtime.RunEnv) context.Context {
	sqonce.Do(func() {
		var err error
		_, sqlogger, err = runenv.CreateStructuredAsset("dht_lookups.out", runtime.StandardJSONConfig())
		if err != nil {
			runenv.RecordMessage("failed to initialize dht_lookups.out asset; nooping logger: %s", err)
			sqlogger = zap.NewNop().Sugar()
		}
	})

	ectx, events := kaddht.RegisterForLookupEvents(ctx)
	log := sqlogger

	go func() {
		for e := range events {
			log.Infow("lookup event", "info", e)
		}
	}()

	return ectx
}

// TableHealth computes health reports for a network of nodes, whose routing contacts are given.
func TableHealth(dht *kaddht.IpfsDHT, peers map[peer.ID]*NodeInfo, ri *RunInfo) {
	// Construct global network view trie
	var kn []key.Key
	knownNodes := trie.New()
	for p, info := range peers {
		if info.Properties.ExpectedServer {
			k := kadPeerID(p)
			kn = append(kn, k)
			knownNodes.Add(k)
		}
	}

	rtPeerIDs := dht.RoutingTable().ListPeers()
	rtPeers := make([]key.Key, len(rtPeerIDs))
	for i, p := range rtPeerIDs {
		rtPeers[i] = kadPeerID(p)
	}

	ri.runenv.RecordMessage("rt: %v | all: %v", rtPeers, kn)
	report := kademlia.TableHealth(kadPeerID(dht.PeerID()), rtPeers, knownNodes)
	ri.runenv.RecordMessage("table health: %s", report.String())

	return
}

func kadPeerID(p peer.ID) key.Key {
	return key.KbucketIDToKey(kbucket.ConvertPeerID(p))
}

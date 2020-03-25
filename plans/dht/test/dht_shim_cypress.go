// +build cypress

package test

import (
	"context"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	"go.uber.org/zap"
	"sync"
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
	log := sqlogger.With("node", node.host.ID().Pretty(), "target", target)

	go func() {
		for e := range events {
			if e.Terminate != nil {
				var msg string
				switch e.Terminate.Reason {
				case kaddht.LookupStopped:
					msg = "stopped"
				case kaddht.LookupCancelled:
					msg = "cancelled"
				case kaddht.LookupStarvation:
					msg = "starvation"
				case kaddht.LookupCompleted:
					msg = "completed"
				}
				log.Infow("lookup termination", "eventID" , e.ID ,"target" , e.Key,  "reason", msg)
			}
			if e.Update != nil {
				log.Infow("update", "eventID", e.ID, "target", e.Key,
					"cause", e.Update.Cause,
					"heard", e.Update.Heard,
					"waiting", e.Update.Waiting,
					"queried", e.Update.Queried,
					"unreachable", e.Update.Unreachable,
					"heardKad", peerIDsToKadIDs(e.Update.Heard),
					"waitingKad", peerIDsToKadIDs(e.Update.Waiting),
					"queriedKad", peerIDsToKadIDs(e.Update.Queried),
					"unreachableKad", peerIDsToKadIDs(e.Update.Unreachable),
					)
			}
		}
	}()

	return ectx
}

func peerIDsToKadIDs(peers []peer.ID) []kbucket.ID {
	kadIDs := make([]kbucket.ID, len(peers))
	for i := range peers {
		kadIDs[i] = kbucket.ConvertPeerID(peers[i])
	}
	return kadIDs
}

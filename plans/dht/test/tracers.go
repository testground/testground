package test

import (
	"context"
	"sync"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	routing "github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/zap"
)

var (
	qonce   sync.Once
	qlogger *zap.SugaredLogger
)

// TraceConnections starts tracing connections into an output asset with name
// conn_trace.out.
func TraceConnections(runenv *runtime.RunEnv, node host.Host) error {
	_, trace, err := runenv.CreateStructuredAsset("conn_trace.out", runtime.StandardJSONConfig())
	if err != nil {
		return err
	}

	trace = trace.With("id", node.ID())

	node.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			trace.Infow("connect", "peer", c.RemotePeer(), "dir", c.Stat().Direction)
		},
		DisconnectedF: func(n network.Network, c network.Conn) {
			trace.Infow("disconnect", "peer", c.RemotePeer())
		},
	})

	return nil
}

// TraceQuery returns a context.Context that can be used in a DHT query to
// cause query events to be traced. It initialises the output asset once.
func TraceQuery(ctx context.Context, runenv *runtime.RunEnv, target string) context.Context {
	qonce.Do(func() {
		var err error
		_, qlogger, err = runenv.CreateStructuredAsset("dht_queries.out", runtime.StandardJSONConfig())
		if err != nil {
			runenv.RecordMessage("failed to initialize dht_queries.out asset; nooping logger: %s", err)
			qlogger = zap.NewNop().Sugar()
		}
	})

	ectx, events := routing.RegisterForQueryEvents(ctx)
	log := qlogger.With("target", target)

	go func() {
		for e := range events {
			var msg string
			switch e.Type {
			case routing.SendingQuery:
				msg = "send"
			case routing.PeerResponse:
				msg = "receive"
			case routing.AddingPeer:
				msg = "adding"
			case routing.DialingPeer:
				msg = "dialing"
			case routing.QueryError:
				msg = "error"
			case routing.Provider, routing.Value:
				msg = "result"
			}
			log.Infow(msg, "peer", e.ID, "closer", e.Responses, "value", e.Extra, "closer", e.Responses)
		}
	}()

	return ectx
}

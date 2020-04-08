package libp2p

import (
	"context"
	"github.com/ipfs/testground/plans/dht/utils"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"reflect"
)

func ShareAddresses(ctx context.Context, ri *utils.RunInfo, nodeInfo *NodeInfo) (map[peer.ID]*NodeInfo, error) {
	otherNodes := make(map[peer.ID]*NodeInfo)

	if _, err := ri.Writer.Write(ctx, PeerAttribSubtree, nodeInfo); err != nil {
		return nil, errors.Wrap(err, "peer attrib writer failure")
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	attribCh := make(chan *NodeInfo)
	if err := ri.Watcher.Subscribe(subCtx, PeerAttribSubtree, attribCh); err != nil {
		return nil, errors.Wrap(err, "peer attrib subscription failure")
	}

	for i := 0; i < ri.RunEnv.TestInstanceCount; i++ {
		select {
		case info := <-attribCh:
			if info.Seq == nodeInfo.Seq {
				continue
			}
			otherNodes[info.Addrs.ID] = info
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return otherNodes, nil
}

type NodeInfo struct {
	Seq      int // sequence number within the test
	GroupSeq int // sequence number within the test group
	Group    string
	Addrs    *peer.AddrInfo
}

// PeerAttribSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var PeerAttribSubtree = &sync.Subtree{
	GroupKey:    "attribs",
	PayloadType: reflect.TypeOf(&NodeInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*NodeInfo).Addrs.ID.Pretty()
	},
}

package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/ipfs/testground/plans/dht/utils"
	"github.com/testground/sdk-go/sync"
)

func ShareAddresses(ctx context.Context, ri *utils.RunInfo, nodeInfo *NodeInfo) (map[peer.ID]*NodeInfo, error) {
	otherNodes := make(map[peer.ID]*NodeInfo)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	attribCh := make(chan *NodeInfo)
	if _, _, err := ri.Client.PublishSubscribe(subCtx, PeerAttribTopic, nodeInfo, attribCh); err != nil {
		return nil, errors.Wrap(err, "peer attrib publish/subscribe failure")
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

// PeerAttribTopic represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var PeerAttribTopic = sync.NewTopic("attribs", &NodeInfo{})

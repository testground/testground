package test

import (
	"github.com/ipfs/testground/sdk/sync"
	"github.com/libp2p/go-libp2p-core/peer"
	"reflect"
)

// PeerIDSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their IDs.
var PeerIDSubtree = &sync.Subtree{
	GroupKey:    "nodeIDs",
	PayloadType: reflect.TypeOf((*peer.ID)(nil)),
	KeyFunc: func(val interface{}) string {
		return val.(*peer.ID).Pretty()
	},
}

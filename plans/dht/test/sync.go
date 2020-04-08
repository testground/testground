package test

import (
	"github.com/ipfs/testground/sdk/sync"
	"reflect"
)

// PeerAttribSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var PeerAttribSubtree = &sync.Subtree{
	GroupKey:    "attribs",
	PayloadType: reflect.TypeOf(&DHTNodeInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*DHTNodeInfo).Addrs.ID.Pretty()
	},
}

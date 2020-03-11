package test

import (
	"github.com/ipfs/testground/sdk/sync"
	"reflect"
)

// GroupIDSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their groups.
var GroupIDSubtree = &sync.Subtree{
	GroupKey:    "groupIDs",
	PayloadType: reflect.TypeOf((*GroupInfo)(nil)),
	KeyFunc: func(val interface{}) string {
		return val.(*GroupInfo).ID
	},
}

type GroupInfo struct {
	ID string
	Size int
}

// PeerAttribSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var PeerAttribSubtree = &sync.Subtree{
	GroupKey:    "attribs",
	PayloadType: reflect.TypeOf((*NodeInfo)(nil)),
	KeyFunc: func(val interface{}) string {
		return val.(*NodeInfo).Addrs.ID.Pretty()
	},
}
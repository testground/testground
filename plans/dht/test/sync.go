package test

import (
	"github.com/ipfs/testground/sdk/sync"
	"reflect"
)

// PeerAttribTopic represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their attributes.
var PeerAttribTopic = &sync.Topic{
	Name: "attribs",
	Type: reflect.TypeOf(&DHTNodeInfo{}),
}

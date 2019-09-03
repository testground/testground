package sync

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/libp2p/go-libp2p-core/peer"

	capi "github.com/hashicorp/consul/api"
)

// PeerSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise themselves.
var PeerSubtree = &Subtree{
	PayloadType: reflect.TypeOf(&peer.AddrInfo{}),
	PathFunc: func(val interface{}) string {
		ai := val.(*peer.AddrInfo)
		return fmt.Sprintf("nodes/%s/addrs", ai.ID.Pretty())
	},
	MatchFunc: func() func(kv *capi.KVPair) bool {
		regexPeers := regexp.MustCompile("\\/nodes\\/.*?/addrs")
		return func(kv *capi.KVPair) bool {
			match := regexPeers.MatchString(kv.Key)
			return match
		}
	}(),
}

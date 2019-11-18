package sync

import (
	"net"
	"reflect"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type Action int

const (
	ActionAdded Action = iota
	ActionDeleted
)

type PeerPayload struct {
	Action
	*peer.AddrInfo
}

// PeerSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise themselves.
var PeerSubtree = &Subtree{
	GroupKey:    "nodes",
	PayloadType: reflect.TypeOf(&peer.AddrInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*peer.AddrInfo).ID.Pretty()
	},
}

// LinkShape defines how traffic should be shaped.
type LinkShape struct {
	// Latency is the egress latency
	Latency time.Duration

	// Bandwidth is egress bytes per second
	Bandwidth uint64

	// Drop all inbound traffic.
	Filter bool
}

// LinkRule applies a LinkShape to a subnet.
type LinkRule struct {
	LinkShape
	Subnet net.IPNet
}

// NetworkConfig specifies how a node's network should be configured.
type NetworkConfig struct {
	// Network is the name of the network to configure
	Network string

	// IP sets the IP address of this network device. If unspecified, docker
	// will assign a random IP address.
	IP *net.IPNet

	// Enable enables this network device.
	Enable bool

	// Default is the default link shaping rule.
	Default LinkShape
	// Rules defines how traffic should be shaped to different subnets.
	Rules []LinkRule

	// State will be signaled when the link changes are applied. Nodes can
	// use the same state to wait for _all_ nodes to enter the desired
	// network state.
	State State
}

// PeerSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise themselves.
func NetworkSubtree(container string) *Subtree {
	return &Subtree{
		GroupKey:    "network:" + container,
		PayloadType: reflect.TypeOf(&NetworkConfig{}),
		KeyFunc: func(val interface{}) string {
			return val.(*NetworkConfig).Network
		},
	}
}

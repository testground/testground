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

type FilterAction int

const (
	Accept FilterAction = iota
	Reject
	Drop
)

// LinkShape defines how traffic should be shaped.
type LinkShape struct {
	// Latency is the egress latency
	Latency time.Duration

	// Jitter is the egress jitter
	Jitter time.Duration

	// Bandwidth is egress bytes per second
	Bandwidth uint64

	// Drop all inbound traffic.
	// TODO: Not implemented
	Filter FilterAction

	// Loss is the egress packet loss (%)
	Loss float32

	// Corrupt is the egress packet corruption probability (%)
	Corrupt float32

	// Corrupt is the egress packet corruption correlation (%)
	CorruptCorr float32

	// Reorder is the probability that an egress packet will be reordered (%)
	//
	// Reordered packets will skip the latency delay and be sent
	// immediately. You must specify a non-zero Latency for this option to
	// make sense.
	Reorder float32

	// ReorderCorr is the egress packet reordering correlation (%)
	ReorderCorr float32

	// Duplicate is the percentage of packets that are duplicated (%)
	Duplicate float32

	// DuplicateCorr is the correlation between egress packet duplication (%)
	DuplicateCorr float32
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

package sync

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

// Deprecated
type FilterAction int

const (
	// Deprecated
	Accept FilterAction = iota
	// Deprecated
	Reject
	// Deprecated
	Drop
)

// LinkShape defines how traffic should be shaped.
//
// Deprecated
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
//
// Deprecated
type LinkRule struct {
	LinkShape
	Subnet net.IPNet
}

// NetworkConfig specifies how a node's network should be configured.
//
// Deprecated
type NetworkConfig struct {
	// Network is the name of the network to configure
	Network string

	// IPv4 and IPv6 set the IP addresses of this network device. If
	// unspecified, the sidecar will leave them alone.
	//
	// Your test-case will be assigned a B block in the range
	// 16.0.0.1-32.0.0.0. X.Y.0.1 will always be reserved for the gateway
	// and shouldn't be used by the test.
	//
	// TODO: IPv6 is currently not supported.
	IPv4, IPv6 *net.IPNet

	// Enable enables this network device.
	Enable bool

	// Default is the default link shaping rule.
	Default LinkShape

	// Rules defines how traffic should be shaped to different subnets.
	// TODO: This is not implemented.
	Rules []LinkRule

	// State will be signaled when the link changes are applied. Nodes can
	// use the same state to wait for _all_ nodes to enter the desired
	// network state.
	State State
}

// NetworkTopic represents a subtree through which tests runs can communicate
// with their sidecar. Use this communication channel to setup the networking.
// Create this structure using NetworkSubtree(hostname)
//
// Deprecated
func NetworkTopic(container string) *Topic {
	return &Topic{
		Name: "network:" + container,
		Type: reflect.TypeOf(&NetworkConfig{}),
	}
}

const (
	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check cluster_k8s.go for more information
	NetworkInitialisationSuccessful = "network initialisation successful"
	NetworkInitialisationFailed     = "network initialisation failed"
)

// WaitNetworkInitialized waits for the sidecar to initialize the network, if
// the sidecar is enabled.
//
// Deprecated
func (c *Client) WaitNetworkInitialized(ctx context.Context, runenv *runtime.RunEnv) error {
	rp := c.extractor(ctx)
	if rp == nil {
		return ErrNoRunParameters
	}

	if rp.TestSidecar {
		b, err := c.Barrier(ctx, "network-initialized", rp.TestInstanceCount)
		if err != nil {
			runenv.RecordMessage(NetworkInitialisationFailed)
			return fmt.Errorf("failed to initialize network: %w", err)
		}
		<-b.C
	}
	runenv.RecordMessage(NetworkInitialisationSuccessful)
	return nil
}

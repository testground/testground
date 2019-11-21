package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	swarm "github.com/libp2p/go-libp2p-swarm"
)

func init() {
	os.Setenv("LIBP2P_TCP_REUSEPORT", "false")
}

type SetupOpts struct {
	Timeout        time.Duration
	RandomWalk     bool
	NFindPeers     int
	BucketSize     int
	AutoRefresh    bool
	NodesProviding int
	RecordCount    int
}

// NewDHTNode creates a libp2p Host, and a DHT instance on top of it.
func NewDHTNode(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts) (host.Host, *kaddht.IpfsDHT, error) {
	swarm.DialTimeoutLocal = opts.Timeout

	node, err := libp2p.New(ctx)
	if err != nil {
		return nil, nil, err
	}

	dhtOptions := []dhtopts.Option{
		dhtopts.Datastore(datastore.NewMapDatastore()),
		dhtopts.BucketSize(opts.BucketSize),
	}

	if !opts.AutoRefresh {
		dhtOptions = append(dhtOptions, dhtopts.DisableAutoRefresh())
	}

	dht, err := kaddht.New(ctx, node, dhtOptions...)
	if err != nil {
		return nil, nil, err
	}
	return node, dht, nil
}

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
func SetupNetwork(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) error {
	if !runenv.TestSidecar {
		return nil
	}
	// TODO: just put the hostname inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// Wait for the network to be ready.
	//
	// Technically, we don't need to do this as configuring the network will
	// block on it being ready.
	err = <-watcher.Barrier(ctx, "network-initialized", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to initialize network: %w", err)
	}

	writer.Write(sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		},
		State: "network-configured",
	})

	err = <-watcher.Barrier(ctx, "network-configured", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

// Setup sets up the elements necessary for the test cases
func Setup(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts) (host.Host, *kaddht.IpfsDHT, []peer.AddrInfo, int64, error) {
	var seq int64

	err := SetupNetwork(ctx, runenv, watcher, writer)
	if err != nil {
		return nil, nil, nil, seq, err
	}

	node, dht, err := NewDHTNode(ctx, runenv, opts)
	if err != nil {
		return nil, nil, nil, seq, err
	}

	id := node.ID()
	runenv.Message("I am %s with addrs: %v", id, node.Addrs())

	if seq, err = writer.Write(sync.PeerSubtree, host.InfoFromHost(node)); err != nil {
		return nil, nil, nil, seq, fmt.Errorf("failed to write peer subtree in sync service: %w", err)
	}

	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
	if err != nil {
		runenv.Abort(err)
		return nil, nil, nil, seq, err
	}
	defer cancelSub()

	var (
		toDial []peer.AddrInfo
		all    []peer.AddrInfo
	)

	// Grab list of other peers that are available for this run.
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:

			// Compute peers to dial.
			// skip over dialing ourselves, and prevent TCP simultaneous
			// connect (known to fail) by only dialing peers whose peer ID
			// is smaller than ours.
			id1, _ := ai.ID.MarshalBinary()
			id2, _ := id.MarshalBinary()

			switch cmp := bytes.Compare(id1, id2); {
			case cmp == 0:
				continue
			case cmp < 0:
				toDial = append(toDial, *ai)
			}
			all = append(all, *ai)

		case <-time.After(opts.Timeout):
			return nil, nil, nil, seq, fmt.Errorf("no new peers in %d seconds", opts.Timeout/time.Second)
		}
	}

	return node, dht, all, seq, nil
}

// Connect connects a host to a set of peers.
func Connect(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT, opts *SetupOpts, toDial ...peer.AddrInfo) error {
	// Dial to all the other peers.
	for _, ai := range toDial {
		if err := dht.Host().Connect(ctx, ai); err != nil {
			return fmt.Errorf("error while dialing peer %v: %w", ai.Addrs, err)
		}
	}

	runenv.Message("dialed %d other peers", len(toDial))
	return nil
}

// RandomWalk performs 5 random walks.
func RandomWalk(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT) error {
	for i := 0; i < 5; i++ {
		if err := dht.Bootstrap(ctx); err != nil {
			return fmt.Errorf("Could not run a random-walk: %w", err)
		}
	}
	return nil
}

// WaitRoutingTable waits until the routing table is not empty.
func WaitRoutingTable(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT) error {
	for {
		if size := dht.RoutingTable().Size(); size > 0 {
			runenv.Message("routing table members: %d", size)
			return nil
		}

		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return fmt.Errorf("got no peers in routing table")
		}
	}
}

// Teardown concludes this test case, waiting for all other instances to reach
// the 'end' state first.
func Teardown(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) {
	// Set a state barrier.
	end := sync.State("end")
	doneCh := watcher.Barrier(ctx, end, int64(runenv.TestInstanceCount))

	// Signal we're done on the end state.
	_, err := writer.SignalEntry(end)
	if err != nil {
		runenv.Abort(err)
		return
	}

	// Wait until all others have signalled.
	if err := <-doneCh; err != nil {
		runenv.Abort(err)
	}
}

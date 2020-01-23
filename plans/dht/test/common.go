package test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/go-datastore"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	tcp "github.com/libp2p/go-tcp-transport"
)

func init() {
	os.Setenv("LIBP2P_TCP_REUSEPORT", "false")
	swarm.BackoffBase = 0
}

const minTestInstances = 16

type SetupOpts struct {
	Timeout        time.Duration
	RandomWalk     bool
	NBootstrap     int
	NFindPeers     int
	BucketSize     int
	AutoRefresh    bool
	NodesProviding int
	RecordCount    int
}

// BootstrapSubtree represents a subtree under the test run's sync tree where
// bootstrap peers advertise themselves.
var BootstrapSubtree = &sync.Subtree{
	GroupKey:    "bootstrap",
	PayloadType: reflect.TypeOf(&peer.AddrInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*peer.AddrInfo).ID.Pretty()
	},
}

var ConnManagerGracePeriod = 1 * time.Second

// NewDHTNode creates a libp2p Host, and a DHT instance on top of it.
func NewDHTNode(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts) (host.Host, *kaddht.IpfsDHT, error) {
	swarm.DialTimeoutLocal = opts.Timeout

	min := int(math.Ceil(math.Log2(float64(runenv.TestInstanceCount)))) * 2
	max := int(float64(min) * 1.1)

	// We need enough connections to be able to trim some and still have a
	// few peers.
	//
	// Note: this check is redundant just to be explicit. If we have over 16
	// peers, we're above this limit.
	if min < 3 || max >= runenv.TestInstanceCount {
		return nil, nil, fmt.Errorf("not enough peers")
	}

	runenv.Message("connmgr parameters: hi=%d, lo=%d", max, min)

	node, err := libp2p.New(
		ctx,
		// Use only the TCP transport without reuseport.
		libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
			tpt := tcp.NewTCPTransport(u)
			tpt.DisableReuseport = true
			return tpt
		}),
		libp2p.DefaultListenAddrs,
		// Setup the connection manager to trim to
		libp2p.ConnectionManager(connmgr.NewConnManager(min, max, ConnManagerGracePeriod)),
	)
	if err != nil {
		return nil, nil, err
	}

	dhtOptions := []dhtopts.Option{
		dhtopts.Datastore(datastore.NewMapDatastore()),
		dhtopts.BucketSize(opts.BucketSize),
		dhtopts.RoutingTableRefreshQueryTimeout(opts.Timeout),
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

	// Wait for the network to be initialized.
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	// TODO: just put the unique testplan id inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return err
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

	// TODO: Take opts.NFindPeers into account when setting a minimum?
	if runenv.TestInstanceCount < minTestInstances {
		return nil, nil, nil, seq, fmt.Errorf(
			"requires at least %d instances, only %d started",
			minTestInstances, runenv.TestInstanceCount,
		)
	}

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
		return nil, nil, nil, seq, err
	}
	defer cancelSub()

	// TODO: remove this if it becomes too much coordination effort.
	peers := make([]peer.AddrInfo, 0, runenv.TestInstanceCount)
	// Grab list of other peers that are available for this run.
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:
			if ai.ID == id {
				continue
			}
			peers = append(peers, *ai)
		case <-ctx.Done():
			return nil, nil, nil, seq, fmt.Errorf("no new peers in %d seconds", opts.Timeout/time.Second)
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})

	return node, dht, peers, seq, nil
}

// Bootstrap brings the network into a completely bootstrapped and ready state.
//
// 1. Connect:
//   a. If any bootstrappers are defined, it connects them together and connects all other peers to one of the bootstrappers (deterministically).
//   b. Otherwise, every peer is connected to the next peer (in lexicographical peer ID order).
// 2. Routing: Refresh all the routing tables.
// 3. Trim: Wait out the grace period then invoke the connection manager to simulate a running network with connection churn.
// 4. Forget & Reconnect:
//   a. Forget the addresses of all peers we've disconnected from. Otherwise, FindPeer is useless.
//   b. Re-connect to at least one node if we've disconnected from _all_ nodes.
//      We may want to make this an error in the future?
func Bootstrap(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts, dht *kaddht.IpfsDHT, peers []peer.AddrInfo, seq int64) error {
	// Are we a bootstrap node?
	isBootstrapper := int(seq) <= opts.NBootstrap

	////////////////
	// 1: CONNECT //
	////////////////

	runenv.Message("bootstrap: begin connect")

	var toDial []peer.AddrInfo
	if opts.NBootstrap > 0 {
		// We have bootstrappers.

		if isBootstrapper {
			runenv.Message("bootstrap: am bootstrapper")
			go func() {
				for {
					select {
					case <-time.After(1 * time.Second):
						runenv.Message("bootstrapper peer count: %d", len(dht.Host().Network().Peers()))
						continue
					case <-ctx.Done():
						return
					}
				}
			}()
			// Announce ourself as a bootstrap node.
			if _, err := writer.Write(BootstrapSubtree, host.InfoFromHost(dht.Host())); err != nil {
				return err
			}
			// NOTE: If we start restricting the network, don't restrict
			// bootstrappers.
		}

		runenv.Message("bootstrap: getting bootstrappers")
		// List all the bootstrappers.
		bootstrapPeers, err := getBootstrappers(ctx, runenv, watcher, opts)
		if err != nil {
			return err
		}

		runenv.Message("bootstrap: got %d bootstrappers", len(bootstrapPeers))

		if isBootstrapper {
			// If we're a bootstrapper, connect to all of them with IDs lexicographically less than us
			toDial = make([]peer.AddrInfo, 0, len(bootstrapPeers))
			for _, b := range bootstrapPeers {
				if b.ID < dht.Host().ID() {
					toDial = append(toDial, b)
				}
			}
		} else {
			// Otherwise, connect to a random one (based on our sequence number).
			toDial = append(toDial, bootstrapPeers[int(seq)%len(bootstrapPeers)])
		}
	} else {
		// No bootstrappers, dial the _next_ peer in the ring. This list
		// is sorted.
		idx := sort.Search(len(peers), func(i int) bool {
			return peers[i].ID > dht.Host().ID()
		}) % len(peers)
		toDial = append(toDial, peers[idx])
	}

	runenv.Message("bootstrap: dialing %v", toDial)

	// Connect to our peers.
	if err := Connect(ctx, runenv, dht, toDial...); err != nil {
		return err
	}

	runenv.Message("bootstrap: dialed %d other peers", len(toDial))

	// Wait for these peers to be added to the routing table.
	if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
		return err
	}

	runenv.Message("bootstrap: have peer in routing table")

	// Wait till everyone is done bootstrapping.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-connected"); err != nil {
		return err
	}

	////////////////
	// 2: ROUTING //
	////////////////

	runenv.Message("bootstrap: begin routing")

	// Setup our routing tables.
	if err := <-dht.RefreshRoutingTable(); err != nil {
		return err
	}

	runenv.Message("bootstrap: table ready")

	// TODO: Repeat this a few times until our tables have stabilized? That
	// _shouldn't_ be necessary.

	// Wait till everyone has full routing tables.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-routing"); err != nil {
		return err
	}

	/////////////
	// 3: TRIM //
	/////////////

	runenv.Message("bootstrap: begin trim")

	// Need to wait for connections to exit the grace period.
	time.Sleep(2 * ConnManagerGracePeriod)

	// Force the connection manager to do it's dirty work. DIE CONNECTIONS
	// DIE!
	dht.Host().ConnManager().TrimOpenConns(ctx)

	// Wait for everyone to finish trimming connections.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-trimmed"); err != nil {
		return err
	}

	///////////////////////////
	// 4: FORGET & RECONNECT //
	///////////////////////////

	// Forget all peers we're no longer connected to. We need to do this
	// _after_ we wait for everyone to trim so we can forget peers that
	// disconnected from us.
	forgotten := 0
	for _, p := range dht.Host().Peerstore().Peers() {
		if dht.Host().Network().Connectedness(p) != network.Connected {
			forgotten++
			dht.Host().Peerstore().ClearAddrs(p)
		}
	}

	runenv.Message("bootstrap: forgotten %d peers", forgotten)

	// Make sure we have at least one peer. If not, reconnect to a
	// bootstrapper and log a warning.
	if len(dht.Host().Network().Peers()) == 0 {
		// TODO: Report this as an error?
		runenv.Message("bootstrap: fully disconnected, reconnecting.")
		if err := Connect(ctx, runenv, dht, toDial...); err != nil {
			return err
		}
		if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
			return err
		}
		runenv.Message("bootstrap: finished reconnecting to %d peers", len(toDial))
	}

	// Wait for everyone to finish trimming connections.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-ready"); err != nil {
		return err
	}

	if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
		return err
	}

	runenv.Message(
		"bootstrap: finished with %d connections, %d in the routing table",
		len(dht.Host().Network().Peers()),
		dht.RoutingTable().Size(),
	)

	runenv.Message("bootstrap: done")
	return nil
}

// get all bootstrap peers.
func getBootstrappers(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, opts *SetupOpts) ([]peer.AddrInfo, error) {
	peerCh := make(chan *peer.AddrInfo, opts.NBootstrap)
	cancelSub, err := watcher.Subscribe(BootstrapSubtree, peerCh)
	if err != nil {
		return nil, err
	}
	defer cancelSub()

	// TODO: remove this if it becomes too much coordination effort.
	peers := make([]peer.AddrInfo, opts.NBootstrap)
	// Grab list of other peers that are available for this run.
	for i := 0; i < opts.NBootstrap; i++ {
		select {
		case ai := <-peerCh:
			peers[i] = *ai
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for bootstrappers")
		}
	}
	runenv.Message("got all bootstrappers: %d", len(peers))
	return peers, nil
}

// Connect connects a host to a set of peers.
//
// Automatically skips our own peer.
func Connect(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT, toDial ...peer.AddrInfo) error {
	tryConnect := func(ctx context.Context, ai peer.AddrInfo, attempts int) error {
		var err error
		for i := 1; i <= attempts; i++ {
			runenv.Message("dialling peer %s (attempt %d)", ai.ID, i)
			select {
			case <-time.After(time.Duration(rand.Intn(500)+100) * time.Millisecond):
			case <-ctx.Done():
				return fmt.Errorf("error while dialing peer %v, attempts made: %d: %w", ai.Addrs, i, ctx.Err())
			}
			if err = dht.Host().Connect(ctx, ai); err == nil {
				return nil
			} else {
				runenv.Message("failed to dial peer %v (attempt %d), err: %s", ai.ID, i, err)
			}
		}
		return fmt.Errorf("failed while dialing peer %v, attempts: %d: %w", ai.Addrs, attempts, err)
	}

	// Dial to all the other peers.
	for _, ai := range toDial {
		if ai.ID == dht.Host().ID() {
			continue
		}
		if err := tryConnect(ctx, ai, 5); err != nil {
			return err
		}
	}

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

// Sync synchronizes all test instances around a single sync point.
func Sync(
	ctx context.Context,
	runenv *runtime.RunEnv,
	watcher *sync.Watcher,
	writer *sync.Writer,
	state sync.State,
) error {
	// Set a state barrier.
	doneCh := watcher.Barrier(ctx, state, int64(runenv.TestInstanceCount))

	// Signal we're in the same state.
	_, err := writer.SignalEntry(state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	return <-doneCh
}

// WaitRoutingTable waits until the routing table is not empty.
func WaitRoutingTable(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT) error {
	for {
		if dht.RoutingTable().Size() > 0 {
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
	err := Sync(ctx, runenv, watcher, writer, "end")
	if err != nil {
		runenv.Abort(err)
	}
}

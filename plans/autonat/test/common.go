package test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"sort"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat-svc"
	relay "github.com/libp2p/go-libp2p-circuit"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	routing "github.com/libp2p/go-libp2p-routing"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	tcp "github.com/libp2p/go-tcp-transport"
)

func init() {
	os.Setenv("LIBP2P_TCP_REUSEPORT", "false")
	swarm.BackoffBase = 0
}

const minTestInstances = 4

type SetupOpts struct {
	Timeout    time.Duration
	NBootstrap int
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

// OrderingSubtree provides a subtree for nodes to agree on their roles.
var OrderingSubtree = &sync.Subtree{
	GroupKey:    "order",
	PayloadType: reflect.TypeOf(&peer.AddrInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*peer.AddrInfo).ID.Pretty()
	},
}

var ConnManagerGracePeriod = 1 * time.Second

type memPeerStoreRouter struct {
	peerstore.Peerstore
}

func (m *memPeerStoreRouter) FindPeer(c context.Context, id peer.ID) (peer.AddrInfo, error) {
	return m.Peerstore.PeerInfo(id), nil
}

// Our mock router implements `ContentRouting` interface to allow autorelay startup.
func (m *memPeerStoreRouter) Provide(_ context.Context, _ cid.Cid, _ bool) error {
	return nil
}

func (m *memPeerStoreRouter) FindProvidersAsync(context.Context, cid.Cid, int) <-chan peer.AddrInfo {
	outchan := make(chan peer.AddrInfo)
	close(outchan)
	return outchan
}

// NewNode creates a libp2p Host
func NewNode(ctx context.Context, runenv *runtime.RunEnv, natted bool, opts *SetupOpts) (host.Host, error) {
	swarm.DialTimeoutLocal = opts.Timeout

	min := int(math.Ceil(math.Log2(float64(runenv.TestInstanceCount)))) * 2
	max := int(float64(min) * 1.1)

	// We need enough connections to be able to trim some and still have a
	// few peers.
	//
	// Note: this check is redundant just to be explicit. If we have over 16
	// peers, we're above this limit.
	if min < 3 || max >= runenv.TestInstanceCount {
		return nil, fmt.Errorf("not enough peers")
	}

	runenv.RecordMessage("connmgr parameters: hi=%d, lo=%d", max, min)

	nodeStore := memPeerStoreRouter{pstoremem.NewPeerstore()}
	makeRouting := func(h host.Host) (routing.PeerRouting, error) {
		return &nodeStore, nil
	}

	tcpAddr, err := getSubnetAddr(runenv.TestSubnet)
	if err != nil {
		return nil, err
	}

	p2pOpts := []libp2p.Option{
		libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
			tpt := tcp.NewTCPTransport(u)
			tpt.DisableReuseport = true
			return tpt
		}),
	}

	if natted {
		tcpAddr.Port = rand.Intn(1024) + 1024
		bogusAddr, err := manet.FromNetAddr(tcpAddr)
		if err != nil {
			return nil, err
		}
		bogusAddrLst := []multiaddr.Multiaddr{bogusAddr}

		p2pOpts = append(p2pOpts,
			libp2p.NoListenAddrs,
			libp2p.AddrsFactory(func(listeningAddrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
				return bogusAddrLst
			}))

		l, err := net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			return nil, err
		}

		go func() {
			for ctx.Err() == nil {
				c, err := l.Accept()
				if err != nil {
					continue
				}
				go func() {
					time.Sleep(time.Second * 5)
					_ = c.Close()
				}()
			}
		}()
	} else {
		p2pOpts = append(p2pOpts, libp2p.DefaultListenAddrs)
	}

	p2pOpts = append(p2pOpts,
		libp2p.EnableRelay(relay.OptDiscovery),
		libp2p.EnableAutoRelay(),
		libp2p.Routing(makeRouting),
		// Setup the connection manager to trim to
		libp2p.ConnectionManager(connmgr.NewConnManager(min, max, ConnManagerGracePeriod)))

	node, err := libp2p.New(
		ctx,
		p2pOpts...,
	)

	if err != nil {
		return nil, err
	}

	_, err = autonat.NewAutoNATService(ctx, node, !natted)

	if err != nil {
		return nil, err
	}

	return node, nil
}

func getSubnetAddr(subnet *runtime.IPNet) (*net.TCPAddr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok {
			if subnet.Contains(ip.IP) {
				tcpAddr := &net.TCPAddr{IP: ip.IP}
				return tcpAddr, nil
			}
		} else {
			panic(fmt.Sprintf("%T", addr))
		}
	}
	return nil, fmt.Errorf("no network interface found. Addrs: %v", addrs)
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

	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		},
		State: "network-configured",
	})
	if err != nil {
		return err
	}

	err = <-watcher.Barrier(ctx, "network-configured", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

func learnSequenceNumber(ctx context.Context, writer *sync.Writer) (seq int64) {
	node, err := libp2p.New(ctx)
	if seq, err = writer.Write(ctx, OrderingSubtree, host.InfoFromHost(node)); err != nil {
		return -1
	}
	return
}

// Setup sets up the elements necessary for the test cases
func Setup(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts) (host.Host, []peer.AddrInfo, int64, error) {
	seq := learnSequenceNumber(ctx, writer)

	// TODO: Take opts.NFindPeers into account when setting a minimum?
	if runenv.TestInstanceCount < minTestInstances {
		return nil, nil, seq, fmt.Errorf(
			"requires at least %d instances, only %d started",
			minTestInstances, runenv.TestInstanceCount,
		)
	}

	isBootstrapper := int(seq) <= opts.NBootstrap
	err := SetupNetwork(ctx, runenv, watcher, writer)
	if err != nil {
		return nil, nil, seq, err
	}

	node, err := NewNode(ctx, runenv, !isBootstrapper, opts)
	if err != nil {
		return nil, nil, seq, err
	}

	id := node.ID()
	runenv.RecordMessage("I am %s with addrs: %v", id, node.Addrs())

	if _, err = writer.Write(ctx, sync.PeerSubtree, host.InfoFromHost(node)); err != nil {
		return nil, nil, seq, fmt.Errorf("failed to write peer subtree in sync service: %w", err)
	}

	peerCh := make(chan *peer.AddrInfo, 16)
	sctx, cancelSub := context.WithCancel(ctx)
	if err := watcher.Subscribe(sctx, sync.PeerSubtree, peerCh); err != nil {
		cancelSub()
		return nil, nil, seq, err
	}
	defer cancelSub()

	// TODO: remove this if it becomes too much coordination effort.
	peers := make([]peer.AddrInfo, 0, runenv.TestInstanceCount)
	// Grab list of other peers that are available for this run.
	for i := 0; i < runenv.TestInstanceCount; i++ {
		ai, ok := <-peerCh
		if !ok {
			return nil, nil, seq, fmt.Errorf("no new peers in %d seconds", opts.Timeout/time.Second)
		}
		if ai.ID == id {
			continue
		}
		peers = append(peers, *ai)
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})

	return node, peers, seq, nil
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
func Bootstrap(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, h host.Host, opts *SetupOpts, peers []peer.AddrInfo, seq int64) ([]peer.AddrInfo, error) {
	// Are we a bootstrap node?
	isBootstrapper := int(seq) <= opts.NBootstrap

	runenv.RecordMessage("bootstrap: begin connect")

	var bootstrapPeers []peer.AddrInfo
	if opts.NBootstrap > 0 {
		// We have bootstrappers.

		if isBootstrapper {
			runenv.RecordMessage("bootstrap: am bootstrapper")
			// Announce ourself as a bootstrap node.
			if _, err := writer.Write(ctx, BootstrapSubtree, host.InfoFromHost(h)); err != nil {
				return nil, err
			}
			// NOTE: If we start restricting the network, don't restrict
			// bootstrappers.
		}

		runenv.RecordMessage("bootstrap: getting bootstrappers")
		// List all the bootstrappers.
		var err error
		bootstrapPeers, err = getBootstrappers(ctx, runenv, watcher, opts)
		if err != nil {
			return nil, err
		}
	} else {
		bootstrapPeers = peers
	}

	// Load peers into peer store.
	for _, p := range peers {
		h.Peerstore().AddAddrs(p.ID, p.Addrs, time.Hour)
	}

	// Wait for everyone to finish.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-done"); err != nil {
		return nil, err
	}

	return bootstrapPeers, nil
}

// get all bootstrap peers.
func getBootstrappers(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, opts *SetupOpts) ([]peer.AddrInfo, error) {
	// cancel the sub
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	peerCh := make(chan *peer.AddrInfo, opts.NBootstrap)
	if err := watcher.Subscribe(ctx, BootstrapSubtree, peerCh); err != nil {
		return nil, err
	}

	// TODO: remove this if it becomes too much coordination effort.
	peers := make([]peer.AddrInfo, opts.NBootstrap)
	// Grab list of other peers that are available for this run.
	for i := 0; i < opts.NBootstrap; i++ {
		ai, ok := <-peerCh
		if !ok {
			return peers, fmt.Errorf("timed out waiting for bootstrappers")
		}
		peers[i] = *ai
	}
	runenv.RecordMessage("got all bootstrappers: %d", len(peers))
	return peers, nil
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
	_, err := writer.SignalEntry(ctx, state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	return <-doneCh
}

// Teardown concludes this test case, waiting for all other instances to reach
// the 'end' state first.
func Teardown(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) {
	err := Sync(ctx, runenv, watcher, writer, "end")
	if err != nil {
		runenv.RecordFailure(fmt.Errorf("end sync failed: %w", err))
	}
}

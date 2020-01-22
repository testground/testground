package test

import (
	"context"
	"fmt"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	gosync "sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/ipfs/go-datastore"

	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	autonatsvc "github.com/libp2p/go-libp2p-autonat-svc"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	tcp "github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-net"
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
	FUndialable    float64
	ClientMode     bool
}

type NodeProperty int

const (
	Undefined NodeProperty = iota
	Bootstrapper
	Undialable
)

type NodeParams struct {
	host host.Host
	dht  *kaddht.IpfsDHT
	info *NodeInfo
}

type NodeInfo struct {
	seq        int
	properties map[NodeProperty]struct{}
	addrs      *peer.AddrInfo
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

type ModeSwitcher interface {
	SetMode(int) error
}

// NewDHTNode creates a libp2p Host, and a DHT instance on top of it.
func NewDHTNode(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts, idKey crypto.PrivKey, params *NodeParams) (host.Host, *kaddht.IpfsDHT, error) {
	swarm.DialTimeoutLocal = opts.Timeout

	_, undialable := params.info.properties[Undialable]
	_, bootstrap := params.info.properties[Bootstrapper]

	var min, max int

	if bootstrap {
		min = runenv.TestInstanceCount
		max = runenv.TestInstanceCount
	} else {
		min = int(math.Ceil(math.Log2(float64(runenv.TestInstanceCount))) * 5)
		max = int(float64(min) * 1.1)
	}

	// We need enough connections to be able to trim some and still have a
	// few peers.
	//
	// Note: this check is redundant just to be explicit. If we have over 16
	// peers, we're above this limit.
	// 	if min < 3 || max >= runenv.TestInstanceCount {
	if min < 3 {
		return nil, nil, fmt.Errorf("not enough peers")
	}

	runenv.Message("connmgr parameters: hi=%d, lo=%d", max, min)

	// Generate bogus advertising address
	tcpAddr, err := getSubnetAddr(runenv.TestSubnet)
	if err != nil {
		return nil, nil, err
	}

	libp2pOpts := []libp2p.Option{
		libp2p.Identity(idKey),
		// Use only the TCP transport without reuseport.
		libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
			tpt := tcp.NewTCPTransport(u)
			tpt.DisableReuseport = true
			return tpt
		}),
		// Setup the connection manager to trim to
		libp2p.ConnectionManager(connmgr.NewConnManager(min, max, ConnManagerGracePeriod)),
	}

	if undialable {
		tcpAddr.Port = rand.Intn(1024) + 1024
		bogusAddr, err := manet.FromNetAddr(tcpAddr)
		if err != nil {
			return nil, nil, err
		}
		bogusAddrLst := []multiaddr.Multiaddr{bogusAddr}

		libp2pOpts = append(libp2pOpts,
			libp2p.NoListenAddrs,
			libp2p.AddrsFactory(func(listeningAddrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
				return bogusAddrLst
			}))
	} else {
		addr, err := manet.FromNetAddr(tcpAddr)
		if err != nil {
			return nil, nil, err
		}

		libp2pOpts = append(libp2pOpts,
			libp2p.ListenAddrs(addr))
	}

	node, err := libp2p.New(ctx, libp2pOpts...)
	if err != nil {
		return nil, nil, err
	}

	if _, err = autonatsvc.NewAutoNATService(ctx, node); err != nil {
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

	if undialable && opts.ClientMode {
		dhtOptions = append(dhtOptions, dhtopts.Client(true))
	}

	dht, err := kaddht.New(ctx, node, dhtOptions...)
	if err != nil {
		return nil, nil, err
	}
	return node, dht, nil
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

	writer.Write(sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   0 * time.Millisecond,
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

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
func SetupNetwork2(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) error {
	if !runenv.TestSidecar {
		return nil
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
		State: "network-configured2",
	})

	err = <-watcher.Barrier(ctx, "network-configured2", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

// Setup sets up the elements necessary for the test cases
func Setup(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts) (*NodeParams, map[peer.ID]*NodeInfo, error) {
	testNode := &NodeParams{info: &NodeInfo{}}
	otherNodes := make(map[peer.ID]*NodeInfo)

	// TODO: Take opts.NFindPeers into account when setting a minimum?
	if runenv.TestInstanceCount < minTestInstances {
		return nil, nil, fmt.Errorf(
			"requires at least %d instances, only %d started",
			minTestInstances, runenv.TestInstanceCount,
		)
	}

	err := SetupNetwork(ctx, runenv, watcher, writer)
	if err != nil {
		return nil, nil, err
	}

	// Set a state barrier.
	seqNumCh := watcher.Barrier(ctx, "seqNum", int64(runenv.TestInstanceCount))

	// Signal we're in the same state.
	seqSeed, err := writer.SignalEntry("seqNum")
	if err != nil {
		return nil, nil, err
	}

	// Wait until all others have signalled.
	if err := <-seqNumCh; err != nil {
		return nil, nil, err
	}

	rng := rand.New(rand.NewSource(int64(seqSeed)))
	priv, _, err := crypto.GenerateEd25519Key(rng)
	if err != nil {
		return nil, nil, err
	}

	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	if _, err = writer.Write(PeerIDSubtree, &id); err != nil {
		return nil, nil, fmt.Errorf("failed to write peer id subtree in sync service: %w", err)
	}

	peerIDCh := make(chan *peer.ID, 16)
	cancelSub, err := watcher.Subscribe(PeerIDSubtree, peerIDCh)
	if err != nil {
		return nil, nil, err
	}
	defer cancelSub()

	// TODO: remove this if it becomes too much coordination effort.
	// Grab list of other peers that are available for this run.
	allPeerIDs := make([]string, runenv.TestInstanceCount)
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case p := <-peerIDCh:
			allPeerIDs[i] = string(*p)
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("no new peers in %d seconds", opts.Timeout/time.Second)
		}
	}

	sort.Strings(allPeerIDs)
	for i, p := range allPeerIDs {
		if peer.ID(p) == id {
			testNode.info.seq = i
			testNode.info.properties = getNodeProperties(i, runenv.TestInstanceCount, opts)
			continue
		}
		otherNodes[peer.ID(p)] = &NodeInfo{
			seq:        i,
			properties: getNodeProperties(i, runenv.TestInstanceCount, opts),
		}
	}

	testNode.host, testNode.dht, err = NewDHTNode(ctx, runenv, opts, priv, testNode)
	if err != nil {
		return nil, nil, err
	}
	testNode.info.addrs = host.InfoFromHost(testNode.host)
	if err != nil {
		return nil, nil, err
	}

	runenv.Message("I am %s with addrs: %v", id, testNode.info.addrs)

	if _, err = writer.Write(sync.PeerSubtree, testNode.info.addrs); err != nil {
		return nil, nil, fmt.Errorf("failed to write peer subtree in sync service: %w", err)
	}

	peerCh := make(chan *peer.AddrInfo, 16)
	cancelSub, err = watcher.Subscribe(sync.PeerSubtree, peerCh)
	if err != nil {
		return nil, nil, err
	}
	defer cancelSub()

	// TODO: remove this if it becomes too much coordination effort.
	// Grab list of other peers that are available for this run.
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case ai := <-peerCh:
			if ai.ID == id {
				continue
			}
			otherNodes[ai.ID].addrs = ai
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("no new peers in %d seconds", opts.Timeout/time.Second)
		}
	}

	if testNode.info.seq == 0 {
		m := make(map[peer.ID]bool)
		for _, info := range otherNodes {
			_, undialable := info.properties[Undialable]
			m[info.addrs.ID] = undialable
		}

		runenv.Message("%v", m)
	}

	return testNode, otherNodes, nil
}

func getNodeProperties(seq, total int, opts *SetupOpts) map[NodeProperty]struct{} {
	properties := make(map[NodeProperty]struct{})
	nb := opts.NBootstrap
	nb = 0
	if seq < nb {
		properties[Bootstrapper] = struct{}{}
	} else {
		numNonBootstrap := total
		if nb > 0 {
			numNonBootstrap -= nb
		}
		if opts.FUndialable > 0 {
			if int(float64(seq)/opts.FUndialable) < numNonBootstrap {
				properties[Undialable] = struct{}{}
			}
		}
	}
	return properties
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
func Bootstrap(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts, node *NodeParams, peers map[peer.ID]*NodeInfo) error {
	// Are we a bootstrap node?
	_, isBootstrapper := node.info.properties[Bootstrapper]
	_, isUndialable := node.info.properties[Undialable]

	////////////////
	// 1: CONNECT //
	////////////////

	runenv.Message("bootstrap: begin connect")

	dht := node.dht

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
			toDial = append(toDial, bootstrapPeers[node.info.seq%len(bootstrapPeers)])
		}
	} else {
		switch {
		case opts.NBootstrap == 0:
			// No bootstrappers, dial the _next_ peer in the ring

			mySeqNo := node.info.seq
			var targetSeqNo int
			if mySeqNo == runenv.TestInstanceCount-1 {
				targetSeqNo = 0
			} else {
				targetSeqNo = mySeqNo + 1
			}
			// look for the node with sequence number 0
			for _, info := range peers {
				if info.seq == targetSeqNo {
					toDial = append(toDial, *info.addrs)
					break
				}
			}
		case opts.NBootstrap == -1:
			// Create mesh of peers
			if _, undialable := node.info.properties[Undialable]; undialable {
				toDial = make([]peer.AddrInfo, 0, len(peers))
				for _, info := range peers {
					if _, undialable := info.properties[Undialable]; !undialable {
						toDial = append(toDial, *info.addrs)
					}
				}
			} else {
				toDial = make([]peer.AddrInfo, 0, len(peers))
				for p, info := range peers {
					if _, undialable := info.properties[Undialable]; !undialable && p < dht.Host().ID() {
						toDial = append(toDial, *info.addrs)
					}
				}
			}
		case opts.NBootstrap == -2:
			/*
				Connect to log(n) of the network and then bootstrap
			*/
			targetSize := int(math.Log2(float64(len(peers)+1)) / 2)
			plist := make([]*NodeInfo, len(peers)+1)
			for _, info := range peers {
				plist[info.seq] = info
			}
			plist[node.info.seq] = node.info

			rng := rand.New(rand.NewSource(0))
			rng.Shuffle(len(plist), func(i, j int) {
				plist[i], plist[j] = plist[j], plist[i]
			})

			index := -1
			for i, info := range plist {
				if info.seq == node.info.seq {
					index = i
				}
			}

			minIndex := index - targetSize
			maxIndex := index + targetSize

			switch {
			case minIndex < 0:
				wrapIndex := len(plist) + minIndex
				minIndex = 0

				for _, info := range plist[wrapIndex:] {
					if _, undialable := info.properties[Undialable]; !undialable && info.addrs.ID < dht.Host().ID() {
						toDial = append(toDial, *info.addrs)
					}
				}
			case maxIndex > len(plist):
				wrapIndex := maxIndex - len(plist) + 1
				maxIndex = len(plist)

				for _, info := range plist[:wrapIndex] {
					if _, undialable := info.properties[Undialable]; !undialable && info.addrs.ID < dht.Host().ID() {
						toDial = append(toDial, *info.addrs)
					}
				}
			}

			for _, info := range plist[minIndex:index] {
				if _, undialable := info.properties[Undialable]; !undialable && info.addrs.ID < dht.Host().ID() {
					toDial = append(toDial, *info.addrs)
				}
			}

			for _, info := range plist[index+1 : maxIndex] {
				if _, undialable := info.properties[Undialable]; !undialable && info.addrs.ID < dht.Host().ID() {
					toDial = append(toDial, *info.addrs)
				}
			}
		case opts.NBootstrap == -3:
			/*
				Connect to log(n) of the network and then bootstrap
			*/
			plist := make([]*NodeInfo, len(peers)+1)
			for _, info := range peers {
				plist[info.seq] = info
			}
			plist[node.info.seq] = node.info
			targetSize := int(math.Log2(float64(len(plist))))

			getRandomNodes := func(info *NodeInfo) map[int]*NodeInfo {
				nodeLst := make([]*NodeInfo, len(plist))
				copy(nodeLst, plist)
				rng := rand.New(rand.NewSource(0))
				rng = rand.New(rand.NewSource(int64(rng.Int31()) + int64(node.info.seq)))
				rng.Shuffle(len(nodeLst), func(i, j int) {
					nodeLst[i], nodeLst[j] = nodeLst[j], nodeLst[i]
				})

				foundSelf := false
				nodes := make(map[int]*NodeInfo)
				for _, n := range nodeLst[:targetSize] {
					if n.seq == info.seq {
						foundSelf = true
						continue
					}
					nodes[n.seq] = n
				}
				if foundSelf {
					replacement := nodeLst[targetSize]
					nodes[replacement.seq] = replacement
				}

				return nodes
			}

			candidateNodes := getRandomNodes(node.info)

			// Because of TCP simultaneous connect issues we need to figure out if anyone will be dialing us.
			// If they are then deterministically only one of us should

			if !isUndialable {
				for _, info := range candidateNodes {
					candidateDialList := getRandomNodes(info)
					if _, ok := candidateDialList[node.info.seq]; ok && dht.Host().ID() < info.addrs.ID {
						delete(candidateNodes, info.seq)
					}
				}
			}

			for _, info := range candidateNodes {
				if _, undialable := info.properties[Undialable]; !undialable {
					toDial = append(toDial, *info.addrs)
				}
			}
		default:
			return fmt.Errorf("invalid number of bootstrappers %d", opts.NBootstrap)
		}
	}

	runenv.Message("bootstrap: dialing %v", toDial)

	// Connect to our peers.
	if err := Connect(ctx, runenv, dht, toDial...); err != nil {
		return err
	}

	runenv.Message("bootstrap: dialed %d other peers", len(toDial))

	dead := isUndialable && len(toDial) == 0
	if !dead {
		// Wait for these peers to be added to the routing table.
		if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
			return err
		}
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

	outputGraph(node.dht, runenv, "br")

	if !dead {
		// Setup our routing tables.
		if err := <-dht.RefreshRoutingTable(); err != nil {
			return err
		}
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

	outputGraph(node.dht, runenv, "bt")

	// Need to wait for connections to exit the grace period.
	time.Sleep(2 * ConnManagerGracePeriod)

	// Force the connection manager to do it's dirty work. DIE CONNECTIONS
	// DIE!
	dht.Host().ConnManager().TrimOpenConns(ctx)

	// Wait for everyone to finish trimming connections.
	if err := Sync(ctx, runenv, watcher, writer, "bootstrap-trimmed"); err != nil {
		return err
	}

	outputGraph(node.dht, runenv, "at")

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
	if len(dht.Host().Network().Peers()) == 0 && !dead {
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

	if !dead {
		if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
			return err
		}
	}

	runenv.Message(
		"bootstrap: finished with %d connections, %d in the routing table",
		len(dht.Host().Network().Peers()),
		dht.RoutingTable().Size(),
	)

	runenv.Message("bootstrap: done")
	return nil
}

func StagedBootstrap(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, opts *SetupOpts, node *NodeParams, peers map[peer.ID]*NodeInfo) error {
	_, isUndialable := node.info.properties[Undialable]
	_ = isUndialable

	stager := &Stager{
		ctx:     ctx,
		seq:     node.info.seq,
		total:   runenv.TestInstanceCount,
		name:    "bootstrapping",
		watcher: watcher,
		writer:  writer,
	}

	////////////////
	// 1: CONNECT //
	////////////////

	runenv.Message("bootstrap: begin connect")

	dht := node.dht

	var toDial []peer.AddrInfo

	/*
		Connect to log(n) of the network and then bootstrap
	*/
	plist := make([]*NodeInfo, len(peers)+1)
	for _, info := range peers {
		plist[info.seq] = info
	}
	plist[node.info.seq] = node.info
	targetSize := int(math.Log2(float64(len(plist)))/2) + 1

	nodeLst := make([]*NodeInfo, len(plist))
	copy(nodeLst, plist)
	rng := rand.New(rand.NewSource(0))
	rng = rand.New(rand.NewSource(int64(rng.Int31()) + int64(node.info.seq)))
	rng.Shuffle(len(nodeLst), func(i, j int) {
		nodeLst[i], nodeLst[j] = nodeLst[j], nodeLst[i]
	})

	for _, info := range nodeLst {
		if len(toDial) > targetSize {
			break
		}
		if info.seq != node.info.seq {
			if _, undialable := info.properties[Undialable]; !undialable {
				toDial = append(toDial, *info.addrs)
			}
		}
	}

	// Wait until it's our turn to bootstrap

	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.Message("bootstrap: dialing %v", toDial)

	_ = autonat.NewAutoNAT(ctx, node.host, nil)

	// Connect to our peers.
	if err := Connect(ctx, runenv, dht, toDial...); err != nil {
		return err
	}

	runenv.Message("bootstrap: dialed %d other peers", len(toDial))

	if err := stager.End(); err != nil {
		return err
	}

	////////////////
	// 2: ROUTING //
	////////////////

	if err := stager.Begin(); err != nil {
		return err
	}

	// Wait for these peers to be added to the routing table.
	if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
		return err
	}

	runenv.Message("bootstrap: have peer in routing table")

	runenv.Message("bootstrap: begin routing")

	outputGraph(node.dht, runenv, "br")

	// Setup our routing tables.
	if err := <-dht.RefreshRoutingTable(); err != nil {
		return err
	}

	runenv.Message("bootstrap: table ready")

	// TODO: Repeat this a few times until our tables have stabilized? That
	// _shouldn't_ be necessary.

	if err := stager.End(); err != nil {
		return err
	}

	/////////////
	// 3: TRIM //
	/////////////

	outputGraph(node.dht, runenv, "bt")

	// Need to wait for connections to exit the grace period.
	time.Sleep(2 * ConnManagerGracePeriod)

	if err := stager.Begin(); err != nil {
		return err
	}

	runenv.Message("bootstrap: begin trim")

	// Force the connection manager to do it's dirty work. DIE CONNECTIONS
	// DIE!
	dht.Host().ConnManager().TrimOpenConns(ctx)

	if err := stager.End(); err != nil {
		return err
	}

	outputGraph(node.dht, runenv, "at")

	///////////////////////////
	// 4: FORGET & RECONNECT //
	///////////////////////////

	if err := stager.Begin(); err != nil {
		return err
	}

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

	if err := stager.End(); err != nil {
		return err
	}

	if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
		return err
	}

	outputGraph(node.dht, runenv, "ab")

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
			//select {
			//case <-time.After(time.Duration(rand.Intn(500))*time.Millisecond + 6*time.Second):
			//case <-ctx.Done():
			//	return fmt.Errorf("error while dialing peer %v, attempts made: %d: %w", ai.Addrs, i, ctx.Err())
			//}
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

type Stager struct {
	ctx     context.Context
	seq     int
	total   int
	name    string
	stage   int
	watcher *sync.Watcher
	writer  *sync.Writer
}

func (s *Stager) Begin() error {
	// Wait until it's out turn
	s.stage += 1
	stage := sync.State(s.name + string(s.stage))
	return <-s.watcher.Barrier(s.ctx, stage, int64(s.seq))
}

func (s *Stager) End() error {
	// Signal that we're done
	stage := sync.State(s.name + string(s.stage))
	_, err := s.writer.SignalEntry(stage)
	if err != nil {
		return err
	}

	return <-s.watcher.Barrier(s.ctx, stage, int64(s.total))
}

func WaitMyTurn(
	ctx context.Context,
	runenv *runtime.RunEnv,
	watcher *sync.Watcher,
	writer *sync.Writer,
	state sync.State,
	seq int,
) (func() error, error) {
	// Wait until it's out turn
	err := <-watcher.Barrier(ctx, state, int64(seq))

	return func() error {
		// Signal that we're done
		_, err := writer.SignalEntry(state)
		return err
	}, err
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
		runenv.RecordFailure(fmt.Errorf("end sync failed: %w", err))
	}
}

var graphLogSetup gosync.Once
var graphLogger, rtLogger *zap.SugaredLogger

func outputGraph(dht *kaddht.IpfsDHT, runenv *runtime.RunEnv, graphID string) {
	graphLogSetup.Do(func() {
		var err error
		_, graphLogger, err = runenv.CreateStructuredAsset("dht_graphs.out", runtime.StandardJSONConfig())
		if err != nil {
			runenv.Message("failed to initialize dht_graphs.out asset; nooping logger: %s", err)
			graphLogger = zap.NewNop().Sugar()
		}

		_, rtLogger, err = runenv.CreateStructuredAsset("dht_rt.out", runtime.StandardJSONConfig())
		if err != nil {
			runenv.Message("failed to initialize dht_rt.out asset; nooping logger: %s", err)
			rtLogger = zap.NewNop().Sugar()
		}
	})

	for _, c := range dht.Host().Network().Conns() {
		if c.Stat().Direction == network.DirOutbound {
			graphLogger.Infow(graphID, "From", c.LocalPeer().Pretty(), "To", c.RemotePeer().Pretty())
		}
	}

	for i, b := range dht.RoutingTable().Buckets {
		for _, p := range b.Peers() {
			rtLogger.Infow(graphID, "Node", dht.PeerID().Pretty(), strconv.Itoa(i), p.Pretty())
		}
	}
}

package test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"sort"
	"strconv"
	gosync "sync"
	"time"

	"github.com/pkg/errors"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	leveldb "github.com/ipfs/go-ds-leveldb"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p-xor/kademlia"
	"github.com/libp2p/go-libp2p-xor/key"
	"github.com/libp2p/go-libp2p-xor/trie"
	tcp "github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-net"

	"go.uber.org/zap"
)

func init() {
	os.Setenv("LIBP2P_TCP_REUSEPORT", "false")
	swarm.BackoffBase = 0
}

const minTestInstances = 16

type OptDatastore int

const (
	OptDatastoreMemory OptDatastore = iota
	OptDatastoreLeveldb
)

type SetupOpts struct {
	Timeout     time.Duration
	Latency     time.Duration
	AutoRefresh bool
	RandomWalk  bool

	BucketSize     int
	Alpha          int
	Beta           int
	NDisjointPaths int

	ClientMode bool
	Datastore  OptDatastore

	PeerIDSeed        int
	Bootstrapper      bool
	BootstrapStrategy int
	Undialable        bool
	GroupOrder        int
	ExpectServer      bool
}

type RunInfo struct {
	runenv  *runtime.RunEnv
	watcher *sync.Watcher
	writer  *sync.Writer

	groups     []string
	groupSizes map[string]int
}

func GetCommonOpts(runenv *runtime.RunEnv) *SetupOpts {
	opts := &SetupOpts{
		Timeout:     time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		Latency:     time.Duration(runenv.IntParam("latency")) * time.Millisecond,
		AutoRefresh: runenv.BooleanParam("auto_refresh"),
		RandomWalk:  runenv.BooleanParam("random_walk"),

		BucketSize:     runenv.IntParam("bucket_size"),
		Alpha:          runenv.IntParam("alpha"),
		Beta:           runenv.IntParam("beta"),

		ClientMode: runenv.BooleanParam("client_mode"),
		Datastore:  OptDatastore(runenv.IntParam("datastore")),

		PeerIDSeed:        runenv.IntParam("peer_id_seed"),
		Bootstrapper:      runenv.BooleanParam("bootstrapper"),
		BootstrapStrategy: runenv.IntParam("bs_strategy"),
		Undialable:        runenv.BooleanParam("undialable"),
		GroupOrder:        runenv.IntParam("group_order"),
		ExpectServer:      runenv.BooleanParam("expect_dht"),
	}
	return opts
}

type NodeParams struct {
	host host.Host
	dht  *kaddht.IpfsDHT
	info *NodeInfo
}

type NodeInfo struct {
	Seq        int // sequence number within the test
	GroupSeq   int // sequence number within the test group
	Properties NodeProperties
	Addrs      *peer.AddrInfo
}

type NodeProperties struct {
	Bootstrapper bool
	Undialable   bool
	ExpectedServer  bool
}

var ConnManagerGracePeriod = 1 * time.Second

// NewDHTNode creates a libp2p Host, and a DHT instance on top of it.
func NewDHTNode(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts, idKey crypto.PrivKey, info *NodeInfo) (host.Host, *kaddht.IpfsDHT, error) {
	swarm.DialTimeoutLocal = opts.Timeout

	var min, max int

	if info.Properties.Bootstrapper {
		// TODO: Assumes only 1 bootstrapper group
		min = (runenv.TestInstanceCount / runenv.TestGroupInstanceCount) *
			int(math.Ceil(math.Log2(float64(runenv.TestInstanceCount))))
		max = min * 2
	} else {
		min = int(math.Ceil(math.Log2(float64(runenv.TestInstanceCount))) * 5)
		max = min * 2
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

	runenv.RecordMessage("connmgr parameters: hi=%d, lo=%d", max, min)

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

	if info.Properties.Undialable {
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

		l, err := net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			return nil, nil, err
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
		addr, err := manet.FromNetAddr(tcpAddr)
		if err != nil {
			return nil, nil, err
		}

		libp2pOpts = append(libp2pOpts,
			libp2p.ListenAddrs(addr))
	}

	libp2pOpts = append(libp2pOpts, getTaggedLibp2pOpts(opts, info)...)

	node, err := libp2p.New(ctx, libp2pOpts...)
	if err != nil {
		return nil, nil, err
	}

	var ds datastore.Batching
	switch opts.Datastore {
	case OptDatastoreMemory:
		ds = dssync.MutexWrap(datastore.NewMapDatastore())
	case OptDatastoreLeveldb:
		ds, err = leveldb.NewDatastore("", nil)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("invalid datastore type")
	}

	runenv.RecordMessage("creating DHT")

	dht, err := createDHT(ctx, node, ds, opts, info)
	if err != nil {
		runenv.RecordMessage("creating DHT error %v", err)
		return nil, nil, err
	}
	runenv.RecordMessage("creating DHT successful")
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

var networkSetupNum int
var networkSetupMx gosync.Mutex

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
func SetupNetwork(ctx context.Context, ri *RunInfo, latency time.Duration) error {
	if !ri.runenv.TestSidecar {
		return nil
	}

	networkSetupMx.Lock()
	defer networkSetupMx.Unlock()

	if networkSetupNum == 0 {
		// Wait for the network to be initialized.
		if err := sync.WaitNetworkInitialized(ctx, ri.runenv, ri.watcher); err != nil {
			return err
		}
	}

	networkSetupNum++

	// TODO: just put the unique testplan id inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	state := sync.State(fmt.Sprintf("network-configured-%d", networkSetupNum))

	ri.writer.Write(ctx, sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   latency,
			Bandwidth: 10 << 20, // 10Mib
		},
		State: state,
	})

	ri.runenv.RecordMessage("finished resetting network latency")

	err = <-ri.watcher.Barrier(ctx, state, int64(ri.runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

// Setup sets up the elements necessary for the test cases
func Setup(ctx context.Context, ri *RunInfo, opts *SetupOpts) (*NodeParams, map[peer.ID]*NodeInfo, error) {
	if err := initAssets(ri.runenv); err != nil {
		return nil, nil, err
	}

	// TODO: Take opts.NFindPeers into account when setting a minimum?
	if ri.runenv.TestInstanceCount < minTestInstances {
		return nil, nil, fmt.Errorf(
			"requires at least %d instances, only %d started",
			minTestInstances, ri.runenv.TestInstanceCount,
		)
	}

	err := SetupNetwork(ctx, ri, 0)
	if err != nil {
		return nil, nil, err
	}

	ri.runenv.RecordMessage("past the setup network barrier")

	groupSeq, err := getGroupSeq(ctx, ri)
	if err != nil {
		return nil, nil, err
	}

	if err := setGroupInfo(ctx, ri, opts, groupSeq); err != nil {
		return nil, nil, err
	}

	ri.runenv.RecordMessage("past group info")

	testSeq := getNodeID(ctx, ri, groupSeq)

	ri.runenv.RecordMessage("past nodeid")

	rng := rand.New(rand.NewSource(int64(testSeq)))
	priv, _, err := crypto.GenerateEd25519Key(rng)
	if err != nil {
		return nil, nil, err
	}

	testNode := &NodeParams{
		host: nil,
		dht:  nil,
		info: &NodeInfo{
			Seq:      testSeq,
			GroupSeq: groupSeq,
			Properties: NodeProperties{
				Bootstrapper: opts.Bootstrapper,
				Undialable:   opts.Undialable,
				ExpectedServer:  opts.ExpectServer,
			},
			Addrs: nil,
		},
	}

	testNode.host, testNode.dht, err = NewDHTNode(ctx, ri.runenv, opts, priv, testNode.info)
	if err != nil {
		return nil, nil, err
	}
	testNode.info.Addrs = host.InfoFromHost(testNode.host)

	otherNodes := make(map[peer.ID]*NodeInfo)

	if _, err := ri.writer.Write(ctx, PeerAttribSubtree, testNode.info); err != nil {
		return nil, nil, errors.Wrap(err, "peer attrib writer failure")
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	attribCh := make(chan *NodeInfo)
	if err := ri.watcher.Subscribe(subCtx, PeerAttribSubtree, attribCh); err != nil {
		return nil, nil, errors.Wrap(err, "peer attrib subscription failure")
	}

	for i := 0; i < ri.runenv.TestInstanceCount; i++ {
		select {
		case info := <-attribCh:
			if info.Seq == testNode.info.Seq {
				continue
			}
			otherNodes[info.Addrs.ID] = info
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

	ri.runenv.RecordMessage("finished setup function")

	outputStart(testNode)

	return testNode, otherNodes, nil
}

// getGroupSeq returns the sequence number of this test instance within its group
func getGroupSeq(ctx context.Context, ri *RunInfo) (int, error) {
	// Set a state barrier.
	seqNumCh := ri.watcher.Barrier(ctx, sync.State(ri.runenv.TestGroupID), int64(ri.runenv.TestGroupInstanceCount))

	// Signal we're in the same state.
	seq, err := ri.writer.SignalEntry(ctx, sync.State(ri.runenv.TestGroupID))
	if err != nil {
		return 0, err
	}

	// make sequence number 0 indexed
	seq--

	// Wait until all others have signalled.
	if err := <-seqNumCh; err != nil {
		return 0, err
	}

	return int(seq), nil
}

// setGroupInfo uses the sync service to determine which groups are part of the test and to get their sizes.
// This information is set on the passed in RunInfo.
func setGroupInfo(ctx context.Context, ri *RunInfo, opts *SetupOpts, seq int) error {
	gi := &GroupInfo{
		ID:   ri.runenv.TestGroupID,
		Size: ri.runenv.TestGroupInstanceCount,
	}

	gi.Order = opts.GroupOrder

	if _, err := ri.writer.Write(ctx, GroupIDSubtree, gi); err != nil {
		return err
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	groupInfoCh := make(chan *GroupInfo)
	if err := ri.watcher.Subscribe(subCtx, GroupIDSubtree, groupInfoCh); err != nil {
		return err
	}

	groupOrder := make(map[int][]string)
	groups := make(map[string]int)
	for i := 0; i < ri.runenv.TestInstanceCount; i++ {
		select {
		case g, more := <-groupInfoCh:
			if !more {
				break
			}
			if _, ok := groups[g.ID]; !ok {
				groups[g.ID] = g.Size
				groupOrder[g.Order] = append(groupOrder[g.Order], g.ID)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ri.runenv.RecordMessage("there are %d groups %v", len(groups), groups)

	sortedGroups := make([]string, 0, len(groups))
	sortedOrderNums := make([]int, 0, len(groupOrder))
	for order := range groupOrder {
		sortedOrderNums = append(sortedOrderNums, order)
	}
	sort.Ints(sortedOrderNums)

	for i := 0; i < len(sortedOrderNums); i++ {
		sort.Strings(groupOrder[i])
		sortedGroups = append(sortedGroups, groupOrder[i]...)
	}

	ri.groups = sortedGroups
	ri.groupSizes = groups

	ri.runenv.RecordMessage("sortedGroup order %v", sortedGroups)

	return nil
}

// getNodeID returns the sequence number of this test instance within the test
func getNodeID(ctx context.Context, ri *RunInfo, seq int) int {
	id := seq
	for _, g := range ri.groups {
		if g == ri.runenv.TestGroupID {
			break
		}
		id += ri.groupSizes[g]
	}

	return id
}

func GetBootstrapNodes(opts *SetupOpts, node *NodeParams, peers map[peer.ID]*NodeInfo) []peer.AddrInfo {
	var toDial []peer.AddrInfo
	switch opts.BootstrapStrategy {
	case 0: // Do nothing
		return toDial
	case 1: // Connect to all bootstrappers
		for _, info := range peers {
			if info.Properties.Bootstrapper {
				toDial = append(toDial, *info.Addrs)
			}
		}
	case 2: // Connect to a random bootstrapper (based on our sequence number)
		// List all the bootstrappers.
		var bootstrappers []peer.AddrInfo
		for _, info := range peers {
			if info.Properties.Bootstrapper {
				bootstrappers = append(bootstrappers, *info.Addrs)
			}
		}

		if len(bootstrappers) > 0 {
			toDial = append(toDial, bootstrappers[node.info.Seq%len(bootstrappers)])
		}
	case 3: // Connect to log(n) random bootstrappers (based on our sequence number)
		// List all the bootstrappers.
		var bootstrappers []peer.AddrInfo
		for _, info := range peers {
			if info.Properties.Bootstrapper {
				bootstrappers = append(bootstrappers, *info.Addrs)
			}
		}

		added := make(map[int]struct{})
		if len(bootstrappers) > 0 {
			targetSize := int(math.Log2(float64(len(bootstrappers)))/2) + 1
			rng := rand.New(rand.NewSource(int64(node.info.Seq)))
			for i := 0; i < targetSize; i++ {
				bsIndex := rng.Int() % len(bootstrappers)
				if _, found := added[bsIndex]; found {
					i--
					continue
				}
				toDial = append(toDial, bootstrappers[bsIndex])
			}
		}
	case 4: // dial the _next_ peer in the ring
		mySeqNo := node.info.Seq
		var targetSeqNo int
		if mySeqNo == len(peers) {
			targetSeqNo = 0
		} else {
			targetSeqNo = mySeqNo + 1
		}
		// look for the node with sequence number 0
		for _, info := range peers {
			if info.Seq == targetSeqNo {
				toDial = append(toDial, *info.Addrs)
				break
			}
		}
	case 5: // Connect to all dialable peers
		toDial = make([]peer.AddrInfo, 0, len(peers))
		for _, info := range peers {
			if !info.Properties.Undialable {
				toDial = append(toDial, *info.Addrs)
			}
		}
		return toDial
	case 6: // connect to log(n) of the network
		plist := make([]*NodeInfo, len(peers)+1)
		for _, info := range peers {
			plist[info.Seq] = info
		}
		plist[node.info.Seq] = node.info
		targetSize := int(math.Log2(float64(len(plist)))/2) + 1

		nodeLst := make([]*NodeInfo, len(plist))
		copy(nodeLst, plist)
		rng := rand.New(rand.NewSource(0))
		rng = rand.New(rand.NewSource(int64(rng.Int31()) + int64(node.info.Seq)))
		rng.Shuffle(len(nodeLst), func(i, j int) {
			nodeLst[i], nodeLst[j] = nodeLst[j], nodeLst[i]
		})

		for _, info := range nodeLst {
			if len(toDial) > targetSize {
				break
			}
			if info.Seq != node.info.Seq && !info.Properties.Undialable {
				toDial = append(toDial, *info.Addrs)
			}
		}
	default:
		panic(fmt.Errorf("invalid number of bootstrap strategy %d", opts.BootstrapStrategy))
	}

	return toDial
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
func Bootstrap(ctx context.Context, ri *RunInfo,
	opts *SetupOpts, node *NodeParams, peers map[peer.ID]*NodeInfo, stager Stager, bootstrapNodes []peer.AddrInfo) error {
	runenv := ri.runenv

	defer runenv.RecordMessage("bootstrap phase ended")
	stager.Reset("bootstrapping")

	////////////////
	// 1: CONNECT //
	////////////////

	runenv.RecordMessage("bootstrap: begin connect")

	dht := node.dht

	// Wait until it's our turn to bootstrap
	expGrad := func(seq int) (int, int) {
		switch seq {
		case 0:
			return 0,0
		case 1:
			return 1,1
		default:
			turnNum := int(math.Floor(math.Log2(float64(seq)))) + 1
			waitFor := int(math.Exp2(float64(turnNum - 2)))
			return turnNum, waitFor
		}
	}
	_ = expGrad

	linear := func(seq int) (int,int) {
		slope := 200
		turnNum := int(math.Floor(float64(seq)/float64(slope)))
		waitFor := slope
		if turnNum == 0 {
			waitFor = 0
		}
		return turnNum, waitFor
	}

	gradualBsStager := NewGradualStager(ctx, node.info.Seq, runenv.TestInstanceCount, "boostrap-gradual", ri, linear)
	if err := gradualBsStager.Begin(); err != nil {
		return err
	}

	runenv.RecordMessage("bootstrap: dialing %v", bootstrapNodes)

	// Connect to our peers.
	if err := Connect(ctx, runenv, dht, bootstrapNodes...); err != nil {
		return err
	}

	runenv.RecordMessage("bootstrap: dialed %d other peers", len(bootstrapNodes))

	// TODO: Use an updated autonat that doesn't require this
	// Wait for Autonat to kick in
	time.Sleep(time.Second * 30)

	////////////////
	// 2: ROUTING //
	////////////////

	// Wait for these peers to be added to the routing table.
	if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
		return err
	}

	runenv.RecordMessage("bootstrap: have peer in routing table")

	runenv.RecordMessage("bootstrap: begin routing")

	outputGraph(node.dht, "br")

	rrt := func() error {
		if err := <-dht.RefreshRoutingTable(); err != nil {
			runenv.RecordMessage("bootstrap: refresh failure - rt size %d", dht.RoutingTable().Size())
			outputGraph(dht, "failedrefresh")
			return err
		}
		return nil
	}

	// Setup our routing tables.
	if err := rrt(); err != nil {
		return err
	}

	runenv.RecordMessage("bootstrap: table ready")

	// TODO: Repeat this a few times until our tables have stabilized? That
	// _shouldn't_ be necessary.

	runenv.RecordMessage("bootstrap: everyone table ready")

	outputGraph(node.dht, "ar")

	/////////////
	// 3: TRIM //
	/////////////

	outputGraph(node.dht, "bt")

	// Need to wait for connections to exit the grace period.
	time.Sleep(2 * ConnManagerGracePeriod)

	runenv.Message("bootstrap: begin trim")

	// Force the connection manager to do it's dirty work. DIE CONNECTIONS
	// DIE!
	dht.Host().ConnManager().TrimOpenConns(ctx)

	if err := gradualBsStager.End(); err != nil {
		return err
	}

	outputGraph(node.dht, "at")

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

	runenv.RecordMessage("bootstrap: forgotten %d peers", forgotten)

	// Make sure we have at least one peer. If not, reconnect to a
	// bootstrapper and log a warning.
	if len(dht.Host().Network().Peers()) == 0 {
		// TODO: Report this as an error?
		runenv.RecordMessage("bootstrap: fully disconnected, reconnecting.")
		if err := Connect(ctx, runenv, dht, bootstrapNodes...); err != nil {
			return err
		}
		if err := WaitRoutingTable(ctx, runenv, dht); err != nil {
			return err
		}
		runenv.RecordMessage("bootstrap: finished reconnecting to %d peers", len(bootstrapNodes))
	}

	tmpCtx, tmpc := context.WithTimeout(ctx, time.Second*10)
	if err := WaitRoutingTable(tmpCtx, runenv, dht); err != nil {
		return err
	}
	if tmpCtx.Err() != nil {
		runenv.RecordMessage("peer %s failed with rt of size %d", node.host.ID().Pretty(), node.dht.RoutingTable().Size())
	}
	tmpc()

	if err := stager.End(); err != nil {
		return err
	}

	outputGraph(node.dht, "ab")

	runenv.RecordMessage(
		"bootstrap: finished with %d connections, %d in the routing table",
		len(dht.Host().Network().Peers()),
		dht.RoutingTable().Size(),
	)

	TableHealth(dht, peers, ri)

	runenv.RecordMessage("bootstrap: done")
	return nil
}

// TableHealth computes health reports for a network of nodes, whose routing contacts are given.
func TableHealth(dht *kaddht.IpfsDHT, peers map[peer.ID]*NodeInfo, ri *RunInfo) {
	// Construct global network view trie
	var kn []key.Key
	knownNodes := trie.New()
	for p, info := range peers {
		if info.Properties.ExpectedServer {
			k := kadPeerID(p)
			kn = append(kn , k)
			knownNodes.Add(k)
		}
	}

	rtPeerIDs := dht.RoutingTable().ListPeers()
	rtPeers := make([]key.Key, len(rtPeerIDs))
	for i , p := range rtPeerIDs {
		rtPeers[i] = kadPeerID(p)
	}


	ri.runenv.RecordMessage("rt: %v | all: %v", rtPeers, kn)
	report := kademlia.TableHealth(kadPeerID(dht.PeerID()), rtPeers, knownNodes)
	ri.runenv.RecordMessage("table health: %s", report.String())

	return
}

func kadPeerID(p peer.ID) key.Key {
	return key.Key(kbucket.ConvertPeerID(p))
}

// Connect connects a host to a set of peers.
//
// Automatically skips our own peer.
func Connect(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT, toDial ...peer.AddrInfo) error {
	tryConnect := func(ctx context.Context, ai peer.AddrInfo, attempts int) error {
		var err error
		for i := 1; i <= attempts; i++ {
			runenv.RecordMessage("dialling peer %s (attempt %d)", ai.ID, i)
			if err = dht.Host().Connect(ctx, ai); err == nil {
				return nil
			} else {
				runenv.RecordMessage("failed to dial peer %v (attempt %d), err: %s", ai.ID, i, err)
			}
			select {
			case <-time.After(time.Duration(rand.Intn(5000))*time.Millisecond + 8*time.Second):
			case <-ctx.Done():
				return fmt.Errorf("error while dialing peer %v, attempts made: %d: %w", ai.Addrs, i, ctx.Err())
			}
		}
		return fmt.Errorf("failed while dialing peer %v, attempts: %d: %w", ai.Addrs, attempts, err)
	}

	// Dial to all the other peers.
	var err error
	numFailedConnections := 0
	numAttemptedConnections := 0
	for _, ai := range toDial {
		if ai.ID == dht.Host().ID() {
			continue
		}
		numAttemptedConnections++
		if err = tryConnect(ctx, ai, 10); err != nil {
			numFailedConnections++
		}
	}
	if float64(numFailedConnections)/float64(numAttemptedConnections) > 0.75 {
		return errors.Wrap(err, "too high percentage of failed connections")
	}
	if numAttemptedConnections - numFailedConnections <= 1 {
		return errors.Wrap(err, "insufficient connections formed")
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
	ri *RunInfo,
	state sync.State,
) error {
	// Set a state barrier.
	doneCh := ri.watcher.Barrier(ctx, state, int64(ri.runenv.TestInstanceCount))

	// Signal we're in the same state.
	_, err := ri.writer.SignalEntry(ctx, state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	return <-doneCh
}

type Stager interface {
	Begin() error
	End() error
	Reset(name string)
}

type stager struct {
	ctx     context.Context
	seq     int
	total   int
	name    string
	stage   int
	watcher *sync.Watcher
	writer  *sync.Writer

	re *runtime.RunEnv
	t  time.Time
}

func (s *stager) Reset(name string) {
	s.name = name
	s.stage = 0
}

func NewBatchStager(ctx context.Context, seq int, total int, name string, ri *RunInfo) *BatchStager {
	return &BatchStager{stager{
		ctx:     ctx,
		seq:     seq,
		total:   total,
		name:    name,
		stage:   0,
		watcher: ri.watcher,
		writer:  ri.writer,
		re:      ri.runenv,
		t:       time.Now(),
	}}
}

type BatchStager struct{ stager }

func (s *BatchStager) Begin() error {
	s.stage += 1
	s.t = time.Now()
	return nil
}
func (s *BatchStager) End() error {
	// Signal that we're done
	stage := sync.State(s.name + strconv.Itoa(s.stage))

	t := time.Now()
	_, err := s.writer.SignalEntry(s.ctx, stage)
	if err != nil {
		return err
	}

	s.re.RecordMetric(&runtime.MetricDefinition{
		Name:           "signal " + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	t = time.Now()

	err = <-s.watcher.Barrier(s.ctx, stage, int64(s.total))
	s.re.RecordMetric(&runtime.MetricDefinition{
		Name:           "barrier" + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(t).Nanoseconds()))

	s.re.RecordMetric(&runtime.MetricDefinition{
		Name:           "full " + string(stage),
		Unit:           "ns",
		ImprovementDir: -1,
	}, float64(time.Since(s.t).Nanoseconds()))
	return err
}
func (s *BatchStager) Reset(name string) { s.stager.Reset(name) }

func NewSinglePeerStager(ctx context.Context, seq int, total int, name string, ri *RunInfo) *SinglePeerStager {
	return &SinglePeerStager{BatchStager{stager{
		ctx:     ctx,
		seq:     seq,
		total:   total,
		name:    name,
		stage:   0,
		watcher: ri.watcher,
		writer:  ri.writer,
		re:      ri.runenv,
		t:       time.Now(),
	}}}
}

type SinglePeerStager struct{ BatchStager }

func (s *SinglePeerStager) Begin() error {
	if err := s.BatchStager.Begin(); err != nil {
		return err
	}

	// Wait until it's out turn
	stage := sync.State(s.name + string(s.stage))
	return <-s.watcher.Barrier(s.ctx, stage, int64(s.seq))
}
func (s *SinglePeerStager) End() error {
	return s.BatchStager.End()
}
func (s *SinglePeerStager) Reset(name string) { s.stager.Reset(name) }

func NewGradualStager(ctx context.Context, seq int, total int, name string, ri *RunInfo, gradFn gradualFn) *GradualStager {
	return &GradualStager{BatchStager{stager{
		ctx:     ctx,
		seq:     seq,
		total:   total,
		name:    name,
		stage:   0,
		watcher: ri.watcher,
		writer:  ri.writer,
		re:      ri.runenv,
		t:       time.Now(),
	}}, gradFn}
}

type gradualFn func(seq int) (int, int)

type GradualStager struct {
	BatchStager
	gradualFn
}

func (s *GradualStager) Begin() error {
	if err := s.BatchStager.Begin(); err != nil {
		return err
	}

	// Wait until it's out turn
	ourTurn, waitFor := s.gradualFn(s.seq)

	stageWait := sync.State(fmt.Sprintf("%s%d-%d", s.name, s.stage, ourTurn))
	stageNext := sync.State(fmt.Sprintf("%s%d-%d", s.name, s.stage, ourTurn+1))
	s.re.RecordMessage("%d is waiting on %d from state %d", s.seq, waitFor, ourTurn)
	err := <-s.watcher.Barrier(s.ctx, stageWait, int64(waitFor))
	if err != nil {
		return err
	}
	s.re.RecordMessage("%d is running", s.seq)
	_, err = s.writer.SignalEntry(s.ctx, stageNext)

	return err
}

func (s *GradualStager) End() error {
	lastStage := sync.State(fmt.Sprintf("%s%d-end", s.name, s.stage))
	_, err := s.writer.SignalEntry(s.ctx, lastStage)
	if err != nil {
		return err
	}
	s.re.RecordMessage("%d is done", s.seq)
	err = <-s.watcher.Barrier(s.ctx, lastStage, int64(s.re.TestInstanceCount))
	return err
}

func (s *GradualStager) Reset(name string) { s.stager.Reset(name) }

type NoStager struct{}

func (s *NoStager) Begin() error      { return nil }
func (s *NoStager) End() error        { return nil }
func (s *NoStager) Reset(name string) {}

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
		_, err := writer.SignalEntry(ctx, state)
		return err
	}, err
}

// WaitRoutingTable waits until the routing table is not empty.
func WaitRoutingTable(ctx context.Context, runenv *runtime.RunEnv, dht *kaddht.IpfsDHT) error {
	//ctxt, cancel := context.WithTimeout(ctx, time.Second*10)
	//defer cancel()
	for {
		if dht.RoutingTable().Size() > 0 {
			return nil
		}

		t := time.NewTimer(time.Second * 10)

		select {
		case <-time.After(200 * time.Millisecond):
		case <-t.C:
			runenv.RecordMessage("waiting on routing table")
		case <-ctx.Done():
			peers := dht.Host().Network().Peers()
			errStr := fmt.Sprintf("empty rt. %d peer conns. they are %v", len(peers), peers)
			runenv.RecordMessage(errStr)
			return fmt.Errorf(errStr)
			//return fmt.Errorf("got no peers in routing table")
		}
	}
}

// Teardown concludes this test case, waiting for all other instances to reach
// the 'end' state first.
func Teardown(ctx context.Context, ri *RunInfo) {
	err := Sync(ctx, ri, "end")
	if err != nil {
		ri.runenv.RecordFailure(fmt.Errorf("end sync failed: %w", err))
		panic(err)
	}
}

var graphLogger, rtLogger, nodeLogger *zap.SugaredLogger

func initAssets(runenv *runtime.RunEnv) error {
	var err error
	_, graphLogger, err = runenv.CreateStructuredAsset("dht_graphs.out", runtime.StandardJSONConfig())
	if err != nil {
		runenv.RecordMessage("failed to initialize dht_graphs.out asset; nooping logger: %s", err)
		graphLogger = zap.NewNop().Sugar()
		return err
	}

	_, rtLogger, err = runenv.CreateStructuredAsset("dht_rt.out", runtime.StandardJSONConfig())
	if err != nil {
		runenv.RecordMessage("failed to initialize dht_rt.out asset; nooping logger: %s", err)
		rtLogger = zap.NewNop().Sugar()
		return err
	}

	_, nodeLogger, err = runenv.CreateStructuredAsset("node.out", runtime.StandardJSONConfig())
	if err != nil {
		runenv.RecordMessage("failed to initialize node.out asset; nooping logger: %s", err)
		nodeLogger = zap.NewNop().Sugar()
		return err
	}

	return nil
}

func outputGraph(dht *kaddht.IpfsDHT, graphID string) {
	for _, c := range dht.Host().Network().Conns() {
		if c.Stat().Direction == network.DirOutbound {
			graphLogger.Infow(graphID, "From", c.LocalPeer().Pretty(), "To", c.RemotePeer().Pretty())
		}
	}

	for _, p := range dht.RoutingTable().ListPeers() {
		rtLogger.Infow(graphID, "Node", dht.PeerID().Pretty(), "Peer", p.Pretty())
	}
}

func outputStart(node *NodeParams) {
	nodeLogger.Infow("nodeparams",
		"seq", node.info.Seq,
		"dialable", !node.info.Properties.Undialable,
		"peerID", node.info.Addrs.ID.Pretty(),
		"addrs", node.info.Addrs.Addrs,
	)
}

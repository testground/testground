package test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	gosync "sync"
	"time"

	"github.com/pkg/errors"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	tglibp2p "github.com/ipfs/testground/plans/dht/libp2p"
	"github.com/ipfs/testground/plans/dht/utils"

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
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
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
	ExpectedServer    bool
}

type DHTRunInfo struct {
	*utils.RunInfo
	DHTGroupProperties map[string]*SetupOpts
	Node               *NodeParams
	Others             map[peer.ID]*DHTNodeInfo
}

func GetCommonOpts(runenv *runtime.RunEnv) *SetupOpts {
	opts := &SetupOpts{
		Timeout:     time.Duration(runenv.IntParam("timeout_secs")) * time.Second,
		Latency:     time.Duration(runenv.IntParam("latency")) * time.Millisecond,
		AutoRefresh: runenv.BooleanParam("auto_refresh"),
		RandomWalk:  runenv.BooleanParam("random_walk"),

		BucketSize: runenv.IntParam("bucket_size"),
		Alpha:      runenv.IntParam("alpha"),
		Beta:       runenv.IntParam("beta"),

		ClientMode: runenv.BooleanParam("client_mode"),
		Datastore:  OptDatastore(runenv.IntParam("datastore")),

		PeerIDSeed:        runenv.IntParam("peer_id_seed"),
		Bootstrapper:      runenv.BooleanParam("bootstrapper"),
		BootstrapStrategy: runenv.IntParam("bs_strategy"),
		Undialable:        runenv.BooleanParam("undialable"),
		GroupOrder:        runenv.IntParam("group_order"),
		ExpectedServer:    runenv.BooleanParam("expect_dht"),
	}
	return opts
}

type NodeParams struct {
	host host.Host
	dht  *kaddht.IpfsDHT
	info *DHTNodeInfo
}

type DHTNodeInfo struct {
	*tglibp2p.NodeInfo
	Properties *SetupOpts
}

type NodeProperties struct {
	Bootstrapper   bool
	Undialable     bool
	ExpectedServer bool
}

var ConnManagerGracePeriod = 1 * time.Second

// NewDHTNode creates a libp2p Host, and a DHT instance on top of it.
func NewDHTNode(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts, idKey crypto.PrivKey, info *DHTNodeInfo) (host.Host, *kaddht.IpfsDHT, error) {
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
func SetupNetwork(ctx context.Context, ri *DHTRunInfo, latency time.Duration) error {
	if !ri.RunEnv.TestSidecar {
		return nil
	}
	networkSetupMx.Lock()
	defer networkSetupMx.Unlock()

	if networkSetupNum == 0 {
		// Wait for the network to be initialized.
		if err := ri.Client.WaitNetworkInitialized(ctx, ri.RunEnv); err != nil {
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

	_, _ = ri.Client.Publish(ctx, sync.NetworkTopic(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   latency,
			Bandwidth: 10 << 20, // 10Mib
		},
		State: state,
	})

	ri.RunEnv.RecordMessage("finished resetting network latency")

	err = <-ri.Client.MustBarrier(ctx, state, ri.RunEnv.TestInstanceCount).C
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

// Setup sets up the elements necessary for the test cases
func Setup(ctx context.Context, runenv *runtime.RunEnv, opts *SetupOpts) (*DHTRunInfo, error) {
	if err := initAssets(runenv); err != nil {
		return nil, err
	}

	client := sync.MustBoundClient(ctx, runenv)
	//defer watcher.Close()
	//defer writer.Close()

	ri := &DHTRunInfo{
		RunInfo: &utils.RunInfo{
			RunEnv: runenv,
			Client: client,
		},
		DHTGroupProperties: make(map[string]*SetupOpts),
	}

	// TODO: Take opts.NFindPeers into account when setting a minimum?
	if ri.RunEnv.TestInstanceCount < minTestInstances {
		return nil, fmt.Errorf(
			"requires at least %d instances, only %d started",
			minTestInstances, ri.RunEnv.TestInstanceCount,
		)
	}

	err := SetupNetwork(ctx, ri, 0)
	if err != nil {
		return nil, err
	}

	ri.RunEnv.RecordMessage("past the setup network barrier")

	groupSeq, testSeq, err := utils.GetGroupsAndSeqs(ctx, ri.RunInfo, opts.GroupOrder)
	if err != nil {
		return nil, err
	}

	for g, props := range ri.GroupProperties {
		fakeEnv := &runtime.RunEnv{
			RunParams: runtime.RunParams{TestInstanceParams: props.Params},
		}
		ri.DHTGroupProperties[g] = GetCommonOpts(fakeEnv)
	}

	ri.RunEnv.RecordMessage("past nodeid")

	rng := rand.New(rand.NewSource(int64(testSeq)))
	priv, _, err := crypto.GenerateEd25519Key(rng)
	if err != nil {
		return nil, err
	}

	testNode := &NodeParams{
		host: nil,
		dht:  nil,
		info: &DHTNodeInfo{
			NodeInfo: &tglibp2p.NodeInfo{
				Seq:      testSeq,
				GroupSeq: groupSeq,
				Group:    ri.RunEnv.TestGroupID,
				Addrs:    nil,
			},
			Properties: opts,
		},
	}

	testNode.host, testNode.dht, err = NewDHTNode(ctx, ri.RunEnv, opts, priv, testNode.info)
	if err != nil {
		return nil, err
	}
	testNode.info.Addrs = host.InfoFromHost(testNode.host)

	otherNodes, err := tglibp2p.ShareAddresses(ctx, ri.RunInfo, testNode.info.NodeInfo)
	if err != nil {
		return nil, err
	}

	ri.RunEnv.RecordMessage("finished setup function")

	outputStart(testNode)

	otherDHTNodes := make(map[peer.ID]*DHTNodeInfo, len(otherNodes))
	for pid, nodeInfo := range otherNodes {
		otherDHTNodes[pid] = &DHTNodeInfo{
			NodeInfo:   nodeInfo,
			Properties: ri.DHTGroupProperties[nodeInfo.Group],
		}
	}

	ri.Node = testNode
	ri.Others = otherDHTNodes
	return ri, nil
}

func GetBootstrapNodes(ri *DHTRunInfo) []peer.AddrInfo {
	var toDial []peer.AddrInfo
	nodeInfo := ri.Node.info
	otherNodes := ri.Others

	switch nodeInfo.Properties.BootstrapStrategy {
	case 0: // Do nothing
		return toDial
	case 1: // Connect to all bootstrappers
		for _, info := range otherNodes {
			if info.Properties.Bootstrapper {
				toDial = append(toDial, *info.Addrs)
			}
		}
	case 2: // Connect to a random bootstrapper (based on our sequence number)
		// List all the bootstrappers.
		var bootstrappers []peer.AddrInfo
		for _, info := range otherNodes {
			if info.Properties.Bootstrapper {
				bootstrappers = append(bootstrappers, *info.Addrs)
			}
		}

		if len(bootstrappers) > 0 {
			toDial = append(toDial, bootstrappers[nodeInfo.Seq%len(bootstrappers)])
		}
	case 3: // Connect to log(n) random bootstrappers (based on our sequence number)
		// List all the bootstrappers.
		var bootstrappers []peer.AddrInfo
		for _, info := range otherNodes {
			if info.Properties.Bootstrapper {
				bootstrappers = append(bootstrappers, *info.Addrs)
			}
		}

		added := make(map[int]struct{})
		if len(bootstrappers) > 0 {
			targetSize := int(math.Log2(float64(len(bootstrappers)))/2) + 1
			rng := rand.New(rand.NewSource(int64(nodeInfo.Seq)))
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
		mySeqNo := nodeInfo.Seq
		var targetSeqNo int
		if mySeqNo == len(otherNodes) {
			targetSeqNo = 0
		} else {
			targetSeqNo = mySeqNo + 1
		}
		// look for the node with sequence number 0
		for _, info := range otherNodes {
			if info.Seq == targetSeqNo {
				toDial = append(toDial, *info.Addrs)
				break
			}
		}
	case 5: // Connect to all dialable peers
		toDial = make([]peer.AddrInfo, 0, len(otherNodes))
		for _, info := range otherNodes {
			if !info.Properties.Undialable {
				toDial = append(toDial, *info.Addrs)
			}
		}
		return toDial
	case 6: // connect to log(n) of the network, where n is the number of dialable nodes
		plist := make([]*DHTNodeInfo, len(otherNodes)+1)
		for _, info := range otherNodes {
			plist[info.Seq] = info
		}
		plist[nodeInfo.Seq] = nodeInfo

		numDialable := 0
		for _, info := range plist {
			if !info.Properties.Undialable {
				numDialable++
			}
		}

		targetSize := int(math.Log2(float64(numDialable)/2)) + 1

		nodeLst := make([]*DHTNodeInfo, len(plist))
		copy(nodeLst, plist)
		rng := rand.New(rand.NewSource(0))
		rng = rand.New(rand.NewSource(int64(rng.Int31()) + int64(nodeInfo.Seq)))
		rng.Shuffle(len(nodeLst), func(i, j int) {
			nodeLst[i], nodeLst[j] = nodeLst[j], nodeLst[i]
		})

		for _, info := range nodeLst {
			if len(toDial) > targetSize {
				break
			}
			if info.Seq != nodeInfo.Seq && !info.Properties.Undialable {
				toDial = append(toDial, *info.Addrs)
			}
		}
	case 7: // connect to log(server nodes) and log(other dialable nodes)
		plist := make([]*DHTNodeInfo, len(otherNodes)+1)
		for _, info := range otherNodes {
			plist[info.Seq] = info
		}
		plist[nodeInfo.Seq] = nodeInfo

		numServer := 0
		numOtherDialable := 0
		for _, info := range plist {
			if info.Properties.ExpectedServer {
				numServer++
			} else if !info.Properties.Undialable {
				numOtherDialable++
			}
		}

		targetServerNodes := int(math.Log2(float64(numServer/2))) + 1
		targetOtherNodes := int(math.Log2(float64(numOtherDialable/2))) + 1

		serverAddrs := getBootstrapAddrs(plist, nodeInfo, targetServerNodes, 0, func(info *DHTNodeInfo) bool {
			if info.Seq != nodeInfo.Seq && info.Properties.ExpectedServer {
				return true
			}
			return false
		})

		otherAddrs := getBootstrapAddrs(plist, nodeInfo, targetOtherNodes, 0, func(info *DHTNodeInfo) bool {
			if info.Seq != nodeInfo.Seq && info.Properties.ExpectedServer {
				return true
			}
			return false
		})
		toDial = append(toDial, serverAddrs...)
		toDial = append(toDial, otherAddrs...)
	default:
		panic(fmt.Errorf("invalid number of bootstrap strategy %d", ri.Node.info.Properties.BootstrapStrategy))
	}

	return toDial
}

func getBootstrapAddrs(plist []*DHTNodeInfo, nodeInfo *DHTNodeInfo, targetSize int, rngSeed int64, valid func(info *DHTNodeInfo) bool) (toDial []peer.AddrInfo) {
	nodeLst := make([]*DHTNodeInfo, len(plist))
	copy(nodeLst, plist)
	rng := rand.New(rand.NewSource(rngSeed))
	rng = rand.New(rand.NewSource(int64(rng.Int31()) + int64(nodeInfo.Seq)))
	rng.Shuffle(len(nodeLst), func(i, j int) {
		nodeLst[i], nodeLst[j] = nodeLst[j], nodeLst[i]
	})

	for _, info := range nodeLst {
		if len(toDial) > targetSize {
			break
		}
		if valid(info) {
			toDial = append(toDial, *info.Addrs)
		}
	}
	return
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
func Bootstrap(ctx context.Context, ri *DHTRunInfo, bootstrapNodes []peer.AddrInfo) error {
	runenv := ri.RunEnv

	defer runenv.RecordMessage("bootstrap phase ended")

	node := ri.Node
	dht := node.dht

	stager := utils.NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "bootstrapping", ri.RunInfo)

	////////////////
	// 1: CONNECT //
	////////////////

	runenv.RecordMessage("bootstrap: begin connect")

	// Wait until it's our turn to bootstrap

	gradualBsStager := utils.NewGradualStager(ctx, node.info.Seq, runenv.TestInstanceCount,
		"boostrap-gradual", ri.RunInfo, utils.LinearGradualStaging(100))
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

	if err := Connect(ctx, runenv, dht, bootstrapNodes...); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(time.Second * 5)
		for {
			<-ticker.C
			if node.dht.RoutingTable().Size() < 2 {
				_ = Connect(ctx, runenv, dht, bootstrapNodes...)
			}
		}
	}()

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
			return err
		}
		return nil
	}

	// Setup our routing tables.
	ready := false
	numTries, maxNumTries := 1, 3
	for !ready {
		if err := rrt(); err != nil {
			if numTries >= maxNumTries {
				outputGraph(dht, "failedrefresh")
				return err
			}
			numTries++
			if err := Connect(ctx, runenv, dht, bootstrapNodes...); err != nil {
				return err
			}
		} else {
			ready = true
		}
	}

	runenv.RecordMessage("bootstrap: table ready")

	// TODO: Repeat this a few times until our tables have stabilized? That
	// _shouldn't_ be necessary.

	runenv.RecordMessage("bootstrap: everyone table ready")

	outputGraph(node.dht, "ar")

	/////////////
	// 3: TRIM //
	/////////////

	if err := gradualBsStager.End(); err != nil {
		return err
	}

	// Need to wait for connections to exit the grace period.
	time.Sleep(2 * ConnManagerGracePeriod)

	runenv.RecordMessage("bootstrap: begin trim")

	// Force the connection manager to do it's dirty work. DIE CONNECTIONS
	// DIE!
	dht.Host().ConnManager().TrimOpenConns(ctx)

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
		if dht.RoutingTable().Find(p) == "" && dht.Host().Network().Connectedness(p) != network.Connected {
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
		tmpc()
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

	TableHealth(dht, ri.Others, ri)

	runenv.RecordMessage("bootstrap: done")
	return nil
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
			case <-time.After(time.Duration(rand.Intn(3000))*time.Millisecond + 2*time.Second):
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
		if err = tryConnect(ctx, ai, 3); err != nil {
			numFailedConnections++
		}
	}
	if float64(numFailedConnections)/float64(numAttemptedConnections) > 0.75 {
		return errors.Wrap(err, "too high percentage of failed connections")
	}
	if numAttemptedConnections-numFailedConnections <= 1 {
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

func Base(ctx context.Context, runenv *runtime.RunEnv, commonOpts *SetupOpts) (*DHTRunInfo, error) {
	ectx := specializedTraceQuery(ctx, runenv, "bootstrap-network")
	ri, err := Setup(ectx, runenv, commonOpts)
	if err != nil {
		return nil, err
	}

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ectx, ri, GetBootstrapNodes(ri)); err != nil {
		return nil, err
	}

	if commonOpts.RandomWalk {
		if err = RandomWalk(ectx, runenv, ri.Node.dht); err != nil {
			return nil, err
		}
	}

	if err := SetupNetwork(ectx, ri, commonOpts.Latency); err != nil {
		return nil, err
	}

	return ri, nil
}

// Sync synchronizes all test instances around a single sync point.
func Sync(
	ctx context.Context,
	ri *utils.RunInfo,
	state sync.State,
) error {
	// Set a state barrier.
	doneCh := ri.Client.MustBarrier(ctx, state, ri.RunEnv.TestInstanceCount).C

	// Signal we're in the same state.
	_, err := ri.Client.SignalEntry(ctx, state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	return <-doneCh
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
func Teardown(ctx context.Context, ri *utils.RunInfo) {
	err := Sync(ctx, ri, "end")
	if err != nil {
		ri.RunEnv.RecordFailure(fmt.Errorf("end sync failed: %w", err))
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

package test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

const protocolID = "/testground/secure-channel/transfer"

var (
	metricWriteTime = &runtime.MetricDefinition{Name: "write_time", Unit: "ns", ImprovementDir: -1}
	metricReadTime = &runtime.MetricDefinition{Name: "read_time", Unit: "ns", ImprovementDir: -1}
)

func TestDataTransfer(runenv *runtime.RunEnv) error {
	n, err := makeNode(runenv)
	if err != nil {
		return err
	}

	if n.isInitiator {
		// TODO: better peer selection
		n.initiateTransfer(n.remotePeers[0].ID)
	}

	err = n.waitForAll("end")
	if err != nil {
		n.teardown()
		return fmt.Errorf("error waiting for peers to signal test end: %s", err)
	}

	n.teardown()
	return nil
}

type node struct {
	runenv      *runtime.RunEnv
	syncWatcher *sync.Watcher
	syncWriter  *sync.Writer

	ctx  context.Context
	host host.Host
	teardown func()

	remotePeers []peer.AddrInfo
	isInitiator bool
	payloadSize int64

	payloadSent     bool
	payloadReceived bool
}

func makeNode(runenv *runtime.RunEnv) (*node, error) {
	channelName := runenv.StringParam("secure_channel")
	payloadSize := runenv.IntParam("payload_size")
	timeoutRaw := runenv.IntParam("timeout_secs")
	timeout := time.Duration(timeoutRaw) * time.Second

	watcher, writer := sync.MustWatcherWriter(runenv)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	teardown := func() {
		cancel()
		watcher.Close()
		writer.Close()
	}



	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	if err != nil {
		return nil, fmt.Errorf("error generating keypair: %w", err)
	}
	h, err := newHost(ctx, channelName, priv)
	if err != nil {
		return nil, fmt.Errorf("error constructing libp2p host: %s", err)
	}

	seq, err := writer.Write(sync.PeerSubtree, host.InfoFromHost(h))
	if err != nil {
		return nil, fmt.Errorf("Failed to get Redis Sync PeerSubtree %w", err)
	}

	isInitiator := seq%2 == 0

	runenv.Message("I am %s with addrs: %v. isInitator=%t", h.ID(), h.Addrs(), isInitiator)

	// get addrs for all peers
	peerCh := make(chan *peer.AddrInfo)
	cancelSub, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
	defer cancelSub()
	addrInfos, err := addrInfosFromChan(peerCh, runenv.TestInstanceCount, timeout)
	if err != nil {
		return nil, fmt.Errorf("error getting remote peer addrs: %s", err)
	}

	// add peers to peerstore so we can dial them later
	var remotePeers []peer.AddrInfo
	for _, ai := range addrInfos {
		// ignore ourselves
		if ai.ID == h.ID() {
			continue
		}
		remotePeers = append(remotePeers, ai)
		h.Peerstore().AddAddrs(ai.ID, ai.Addrs, peerstore.RecentlyConnectedAddrTTL)
	}

	n := &node{
		runenv:      runenv,
		syncWatcher: watcher,
		syncWriter:  writer,

		ctx:         ctx,
		host:        h,
		teardown:    teardown,

		remotePeers: remotePeers,
		isInitiator: isInitiator,
		payloadSize: int64(payloadSize),
	}
	h.SetStreamHandler(protocolID, n.handleStream)
	err = n.signalAndWaitForAll("ready")
	if err != nil {
		return nil, fmt.Errorf("error waiting for peers to signal ready state: %s", err)
	}

	return n, nil
}

func (n *node) handleStream(stream core.Stream) {
	remotePeer := stream.Conn().RemotePeer()
	n.runenv.Message("new stream from %s", remotePeer.Pretty())

	start := time.Now()
	c, err := io.Copy(ioutil.Discard, stream)
	if err != nil {
		panic(fmt.Errorf("error reading from remote stream: %v", err))
	}
	if c != n.payloadSize {
		panic(fmt.Errorf("expected to read %d bytes, got %d", n.payloadSize, c))
	}

	elapsed := time.Since(start)
	n.runenv.EmitMetric(metricReadTime, float64(elapsed.Nanoseconds()))

	n.runenv.Message("read %d bytes from %s in %d ms", c, remotePeer.Pretty(), elapsed.Milliseconds())
	n.payloadReceived = true
	if n.payloadSent {
		n.runenv.Message("payload sent and received. signalling test end")
		n.signal("end")
	} else {
		n.runenv.Message("payload received, initiating transfer")
		n.initiateTransfer(remotePeer)
	}
}

func (n *node) initiateTransfer(p peer.ID) {
	n.runenv.Message("initiating transfer to %s", p.Pretty())

	stream, err := n.host.NewStream(n.ctx, p, protocolID)
	if err != nil {
		panic(fmt.Errorf("error opening stream to %s: %s", p.Pretty(), err))
	}

	r := rand.New(rand.NewSource(42))
	start := time.Now()
	lr := io.LimitReader(r, n.payloadSize)

	c, err := io.Copy(stream, lr)
	if err != nil {
		panic(fmt.Errorf("failed to write out bytes: %v", err))
	}
	elapsed := time.Since(start)
	err = stream.Close()
	if err != nil {
		n.runenv.Message("error closing stream: %v", err)
	}

	if err != nil {
		panic(fmt.Errorf("error writing to stream: %s", err))
	}
	if c != n.payloadSize {
		panic(fmt.Errorf("expected to write %d bytes, wrote %d", n.payloadSize, c))
	}

	n.runenv.EmitMetric(metricWriteTime, float64(elapsed.Nanoseconds()))
	n.runenv.Message("wrote %d bytes to %s in %dms", c, p.Pretty(), elapsed.Milliseconds())
	n.payloadSent = true
	if n.payloadReceived {
		n.runenv.Message("payload sent and received. signalling test end")
		n.signal("end")
	} else {
		n.runenv.Message("payload sent, awaiting transfer from remote peer to complete test")
	}
}

func (n *node) signal(stateName string) error {
	// Signal we've entered the state.
	state := sync.State(stateName)
	_, err := n.syncWriter.SignalEntry(state)
	if err != nil {
		return err
	}
	return nil
}

func (n *node) waitForAll(stateName string) error {
	// Set a state barrier.
	state := sync.State(stateName)
	instanceCount := n.runenv.TestInstanceCount
	doneCh := n.syncWatcher.Barrier(n.ctx, state, int64(instanceCount))

	// Wait until all others have signalled.
	if err := <-doneCh; err != nil {
		return err
	}

	return nil
}

func (n *node) signalAndWaitForAll(stateName string) error {
	// Signal we've entered the state.
	err := n.signal(stateName)
	if err != nil {
		return err
	}

	return n.waitForAll(stateName)
}

func addrInfosFromChan(peerCh chan *peer.AddrInfo, count int, timeout time.Duration) ([]peer.AddrInfo, error) {
	var ais []peer.AddrInfo
	for i := 1; i <= count; i++ {
		select {
		case ai := <-peerCh:
			ais = append(ais, *ai)

		case <-time.After(timeout):
			return nil, fmt.Errorf("no new peers in %d seconds", timeout/time.Second)
		}
	}
	return ais, nil
}

package test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	gort "runtime"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/ipfs/go-cid"
	"golang.org/x/sync/errgroup"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/testground/testground/plans/bitswap-tuning/utils"
)

//
// To run use:
//
// ./testground run s bitswap-tuning/fuzz \
//   --builder=exec:go \
//   --runner="local:exec" \
//   --dep="github.com/ipfs/go-bitswap=master" \
//   -instances=8 \
//   --test-param cpuprof_path=/tmp/cpu.prof \
//   --test-param memprof_path=/tmp/mem.prof
//

// Fuzz test Bitswap
func Fuzz(runenv *runtime.RunEnv) error {
	// Test Parameters
	timeout := time.Duration(runenv.IntParam("timeout_secs")) * time.Second
	randomDisconnectsFq := float32(runenv.IntParam("random_disconnects_fq")) / 100
	cpuProfilingEnabled := runenv.IsParamSet("cpuprof_path")
	memProfilingEnabled := runenv.IsParamSet("memprof_path")

	defaultMemProfileRate := gort.MemProfileRate
	if memProfilingEnabled {
		gort.MemProfileRate = 0
	}

	/// --- Set up
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)

	/// --- Tear down
	defer func() {
		_, err := client.SignalAndWait(ctx, "end", runenv.TestInstanceCount)
		if err != nil {
			runenv.RecordFailure(err)
		} else {
			runenv.RecordSuccess()
		}
		defer client.Close()
	}()

	// Create libp2p node
	h, err := libp2p.New(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	peers := sync.NewTopic("peers", &peer.AddrInfo{})

	// Get sequence number of this host
	seq, err := client.Publish(ctx, peers, host.InfoFromHost(h))
	if err != nil {
		return err
	}

	// Get addresses of all peers
	peerCh := make(chan *peer.AddrInfo)
	sctx, cancelSub := context.WithCancel(ctx)
	client.MustSubscribe(sctx, peers, peerCh)

	addrInfos, err := utils.AddrInfosFromChan(peerCh, runenv.TestInstanceCount)
	if err != nil {
		cancelSub()
		return fmt.Errorf("no addrs in %d seconds", timeout/time.Second)
	}
	cancelSub()

	/// --- Warm up

	runenv.RecordMessage("I am %s with addrs: %v", h.ID(), h.Addrs())

	// Set up network (with traffic shaping)
	err = setupFuzzNetwork(ctx, runenv, client)
	if err != nil {
		return fmt.Errorf("Failed to set up network: %w", err)
	}

	// Signal that this node is in the given state, and wait for all peers to
	// send the same signal
	signalAndWaitForAll := func(state string) error {
		_, err := client.SignalAndWait(ctx, sync.State(state), runenv.TestInstanceCount)
		return err
	}

	// Wait for all nodes to be ready to start
	err = signalAndWaitForAll("start")
	if err != nil {
		return err
	}

	runenv.RecordMessage("Starting")
	var bsnode *utils.Node
	rootCidTopic := sync.NewTopic("root-cid", &cid.Cid{})

	// Create a new blockstore
	bstoreDelay := 5 * time.Millisecond
	bstore, err := utils.CreateBlockstore(ctx, bstoreDelay)
	if err != nil {
		return err
	}

	// Create a new bitswap node from the blockstore
	bsnode, err = utils.CreateBitswapNode(ctx, h, bstore)
	if err != nil {
		return err
	}

	// Listen for seed generation
	rootCidCh := make(chan *cid.Cid, 1)
	sctx, cancelRootCidSub := context.WithCancel(ctx)
	defer cancelRootCidSub()
	if _, err := client.Subscribe(sctx, rootCidTopic, rootCidCh); err != nil {
		return fmt.Errorf("Failed to subscribe to rootCidTopic %w", err)
	}

	seedGenerated := sync.State("seed-generated")
	var start time.Time
	// Each peer generates the seed data in series, to avoid
	// overloading a single machine hosting multiple instances
	seedIndex := seq - 1
	if seedIndex > 0 {
		// Wait for the seeds with an index lower than this one
		// to generate their seed data
		doneCh := client.MustBarrier(ctx, seedGenerated, int(seedIndex)).C
		if err = <-doneCh; err != nil {
			return err
		}
	}

	// Generate a file of random size and add it to the datastore
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	fileSize := 2*1024*1024 + rnd.Intn(64*1024*1024)
	runenv.RecordMessage("Generating seed data of %d bytes", fileSize)
	start = time.Now()

	rootCid, err := setupSeed(ctx, runenv, bsnode, fileSize, int(seedIndex))
	if err != nil {
		return fmt.Errorf("Failed to set up seed: %w", err)
	}

	runenv.RecordMessage("Done generating seed data of %d bytes (%s)", fileSize, time.Since(start))

	// Signal we've completed generating the seed data
	_, err = client.SignalEntry(ctx, seedGenerated)
	if err != nil {
		return fmt.Errorf("Failed to signal seed generated: %w", err)
	}

	// Inform other nodes of the root CID
	if _, err = client.Publish(ctx, rootCidTopic, &rootCid); err != nil {
		return fmt.Errorf("Failed to get Redis Sync rootCidTopic %w", err)
	}

	// Get seed cid from all nodes
	var rootCids []cid.Cid
	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case rootCidPtr := <-rootCidCh:
			rootCids = append(rootCids, *rootCidPtr)
		case <-time.After(timeout):
			return fmt.Errorf("could not get all cids in %d seconds", timeout/time.Second)
		}
	}
	cancelRootCidSub()

	// Wait for all nodes to be ready to dial
	err = signalAndWaitForAll("ready-to-connect")
	if err != nil {
		return err
	}

	// Dial all peers
	dialed, err := utils.DialOtherPeers(ctx, h, addrInfos)
	if err != nil {
		return err
	}
	runenv.RecordMessage("Dialed %d other nodes", len(dialed))

	// Wait for all nodes to be connected
	err = signalAndWaitForAll("connect-complete")
	if err != nil {
		return err
	}

	/// --- Start test
	runenv.RecordMessage("Start fetching")

	// Randomly disconnect and reconnect
	var cancelFetchingCtx func()
	if randomDisconnectsFq > 0 {
		var fetchingCtx context.Context
		fetchingCtx, cancelFetchingCtx = context.WithCancel(ctx)
		defer cancelFetchingCtx()
		go func() {
			for {
				time.Sleep(time.Duration(rnd.Intn(1000)) * time.Millisecond)

				select {
				case <-fetchingCtx.Done():
					return
				default:
					// One third of the time, disconnect from a peer then reconnect
					if rnd.Float32() < randomDisconnectsFq {
						conns := h.Network().Conns()
						conn := conns[rnd.Intn(len(conns))]
						runenv.RecordMessage("    closing connection to %s", conn.RemotePeer())
						err := conn.Close()
						if err != nil {
							runenv.RecordMessage("    error disconnecting: %w", err)
						} else {
							ai := peer.AddrInfo{
								ID:    conn.RemotePeer(),
								Addrs: []ma.Multiaddr{conn.RemoteMultiaddr()},
							}
							go func() {
								// time.Sleep(time.Duration(rnd.Intn(200)) * time.Millisecond)
								runenv.RecordMessage("    reconnecting to %s", conn.RemotePeer())
								if err := h.Connect(fetchingCtx, ai); err != nil {
									runenv.RecordMessage("    error while reconnecting to peer %v: %w", ai, err)
								}
								runenv.RecordMessage("    reconnected to %s", conn.RemotePeer())
							}()
						}
					}
				}
			}
		}()
	}

	if cpuProfilingEnabled {
		f, err := os.Create(runenv.StringParam("cpuprof_path") + "." + strconv.Itoa(int(seq)))
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
	}
	if memProfilingEnabled {
		gort.MemProfileRate = defaultMemProfileRate
	}

	g, gctx := errgroup.WithContext(ctx)
	for _, rootCid := range rootCids {
		// Fetch two thirds of the root cids of other nodes
		if rnd.Float32() < 0.3 {
			continue
		}

		rootCid := rootCid
		g.Go(func() error {
			// Stagger the start of the fetch
			startDelay := time.Duration(rnd.Intn(50*runenv.TestInstanceCount)) * time.Millisecond
			time.Sleep(startDelay)

			cidStr := rootCid.String()
			pretty := cidStr[len(cidStr)-6:]

			// Half the time do a regular fetch, half the time cancel and then
			// restart the fetch
			runenv.RecordMessage("  FTCH %s after %s delay", pretty, startDelay)
			start = time.Now()
			cctx, cancel := context.WithCancel(gctx)
			if rnd.Float32() < 0.5 {
				// Cancel after a delay
				go func() {
					cancelDelay := time.Duration(rnd.Intn(100)) * time.Millisecond
					time.Sleep(cancelDelay)
					runenv.RecordMessage("  cancel %s after %s delay", pretty, startDelay)
					cancel()
				}()
				err = bsnode.FetchGraph(cctx, rootCid)
				if err != nil {
					// If there was an error (probably because the fetch was
					// cancelled) try fetching again
					runenv.RecordMessage("  got err fetching %s: %s", pretty, err)
					err = bsnode.FetchGraph(gctx, rootCid)
				}
			} else {
				defer cancel()
				err = bsnode.FetchGraph(cctx, rootCid)
			}
			timeToFetch := time.Since(start)
			runenv.RecordMessage("  RCVD %s in %s", pretty, timeToFetch)

			return err
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("Error fetching data through Bitswap: %w", err)
	}
	runenv.RecordMessage("Fetching complete")
	if randomDisconnectsFq > 0 {
		cancelFetchingCtx()
	}

	// Wait for all leeches to have downloaded the data from seeds
	err = signalAndWaitForAll("transfer-complete")
	if err != nil {
		return err
	}

	if cpuProfilingEnabled {
		pprof.StopCPUProfile()
	}
	if memProfilingEnabled {
		f, err := os.Create(runenv.StringParam("memprof_path") + "." + strconv.Itoa(int(seq)))
		if err != nil {
			return err
		}
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			return err
		}
		f.Close()
	}

	// Shut down bitswap
	err = bsnode.Close()
	if err != nil {
		return fmt.Errorf("Error closing Bitswap: %w", err)
	}

	// Disconnect peers
	for _, c := range h.Network().Conns() {
		err := c.Close()
		if err != nil {
			return fmt.Errorf("Error disconnecting: %w", err)
		}
	}

	/// --- Ending the test

	return nil
}

// Set up traffic shaping with random latency and bandwidth
func setupFuzzNetwork(ctx context.Context, runenv *runtime.RunEnv, client *sync.Client) error {
	if !runenv.TestSidecar {
		return nil
	}

	// Wait for the network to be initialized.
	if err := client.WaitNetworkInitialized(ctx, runenv); err != nil {
		return err
	}

	// TODO: just put the unique testplan id inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	latency := time.Duration(2+rnd.Intn(100)) * time.Millisecond
	bandwidth := 1 + rnd.Intn(100)

	state := sync.State("network-configured")
	topic := sync.NetworkTopic(hostname)
	cfg := &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   latency,
			Bandwidth: uint64(bandwidth * 1024 * 1024),
			Jitter:    (latency * 10) / 100,
		},
		State: state,
	}

	_, err = client.PublishAndWait(ctx, topic, cfg, state, runenv.TestInstanceCount)
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

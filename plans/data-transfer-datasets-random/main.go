package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	utils "github.com/ipfs/testground/plans/data-transfer-datasets-random/utils"
	iptb "github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/libp2p/go-libp2p-core/peer"
	// shell "github.com/ipfs/go-ipfs-api"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/ipfs/go-cid"
)

const (
	// sizeBytes = 10 * 1024 * 1024 // 10mb
	sizeBytes = 100 * 1024 * 1024 // 100mb
)

var cidSubtree = &sync.Subtree{
        GroupKey:    "cid",
        PayloadType: reflect.TypeOf(&cid.Cid{}),
        KeyFunc: func(val interface{}) string {
                return "cid"
        },
}

func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	timeout := func() time.Duration {
                if t, ok := runenv.IntParam("timeout_secs"); !ok {
                        return 30 * time.Second
                } else {
                        return time.Duration(t) * time.Second
                }
        }()

	watcher, writer := sync.MustWatcherWriter(runenv)
        defer watcher.Close()

	spec := iptb.NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(iptb.NodeOpts{Initialize: true, Start: true}, "local")

	ctx := context.Background()
	ensemble := iptb.NewTestEnsemble(ctx, spec)
	ensemble.Initialize()

	localNode := ensemble.GetNode("local")

	peerID, err := localNode.PeerID()
	if err != nil {
		runenv.Abort(err)
	}

	swarmAddrs, err := localNode.SwarmAddrs()
	if err != nil {
		runenv.Abort(err)
	}

	ID, err := peer.IDB58Decode(peerID)
	if err != nil {
		runenv.Abort(err)
	}

	addrs := make([]ma.Multiaddr, len(swarmAddrs))
	for i, addr := range swarmAddrs {
		multiAddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			runenv.Abort(err)
		}
		addrs[i] = multiAddr
	}
	addrInfo := &peer.AddrInfo{ID, addrs}

	seq, err := writer.Write(sync.PeerSubtree, addrInfo)
	if err != nil {
                runenv.Abort(err)
        }
        defer writer.Close()

	client := localNode.Client()

	// States
	added := sync.State("added")
	received := sync.State("received")

	switch {
	case seq == 1: // adder
		adder := client
		fmt.Println("adder")

		// generate a random file of the designated size.
		file := utils.TempRandFile(runenv, ensemble.TempDir(), sizeBytes)
		defer os.Remove(file.Name())

		tstarted := time.Now()
		hash, err := adder.Add(file)
		if err != nil {
			runenv.Abort(err)
			return
		}
		fmt.Println("cid", hash)

		c, err := cid.Parse(hash)
		if err != nil {
			runenv.Abort(err)
			return
		}

		runenv.EmitMetric(utils.MetricTimeToAdd, float64(time.Now().Sub(tstarted)/time.Millisecond))

		_, err = writer.Write(cidSubtree, &c)
		if err != nil {
			runenv.Abort(err)
		}

		// Signal we're done on the added state.
		_, err = writer.SignalEntry(added)
		if err != nil {
			runenv.Abort(err)
		}
		fmt.Println("State: added")

		// Set a state barrier.
		receivedCh := watcher.Barrier(ctx, received, 1)

		// Wait until recieved state is signalled.
		if err := <-receivedCh; err != nil {
			panic(err)
		}
		fmt.Println("State: received")
	case seq == 2: // getter
		getter := client
		fmt.Println("getter")

		// Connect to other peers
		peerCh := make(chan *peer.AddrInfo, 16)
		cancel, err := watcher.Subscribe(sync.PeerSubtree, peerCh)
		if err != nil {
			runenv.Abort(err)
		}
		defer cancel()

		var events int
		for i := 0; i < runenv.TestInstanceCount; i++ {
			select {
			case ai := <-peerCh:
				events++
				if ai.ID == ID {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				tstarted := time.Now()
				err = getter.SwarmConnect(ctx, ai.Addrs[0].String())
				if err != nil {
					runenv.Abort(err)
					return
				}

				runenv.EmitMetric(utils.MetricTimeToConnect, float64(time.Now().Sub(tstarted)/time.Millisecond))
				cancel()

			case <-time.After(timeout):
				// TODO need a way to fail a distributed test immediately. No point
				// making it run elsewhere beyond this point.
				panic(fmt.Sprintf("no new peers in %d seconds", timeout))
			}
		}

		// Set a state barrier.
		addedCh := watcher.Barrier(ctx, added, 1)

		// Wait until added state is signalled.
		if err := <-addedCh; err != nil {
			panic(err)
		}
		fmt.Println("State: added")

		cidCh := make(chan *cid.Cid, 0)
		cancel, err = watcher.Subscribe(cidSubtree, cidCh)
		if err != nil {
			runenv.Abort(err)
		}
		defer cancel()
		select {
		case c := <-cidCh:
			cancel()
			// Get the content from the adder node
			tstarted := time.Now()
			err = getter.Get(c.String(), ensemble.TempDir())
			if err != nil {
				runenv.Abort(err)
				return
			}
			runenv.EmitMetric(utils.MetricTimeToGet, float64(time.Now().Sub(tstarted)/time.Millisecond))
		case <-time.After(timeout):
			// TODO need a way to fail a distributed test immediately. No point
			// making it run elsewhere beyond this point.
			panic(fmt.Sprintf("no cid in %d seconds", timeout))
		}

		// Signal we're reached the received state.
		_, err = writer.SignalEntry(received)
		if err != nil {
			runenv.Abort(err)
		}
		fmt.Println("State: received")
	default:
		runenv.Abort(fmt.Errorf("Unexpected seq: %v", seq))
	}

	ensemble.Destroy()
}


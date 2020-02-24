package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func main() {
	runtime.Invoke(run)
}

var peerID string

var peerIDSubtree = &sync.Subtree{
	GroupKey:    "peerID",
	PayloadType: reflect.TypeOf(&peerID),
	KeyFunc: func(val interface{}) string {
		return "PeerID"
	},
}

func run(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	withShaping := runenv.TestCaseSeq == 1

	if withShaping && !runenv.TestSidecar {
		return fmt.Errorf("Need sidecar to shape traffic")
	}

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	runenv.RecordMessage("Waiting for network to be initialized")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}
	runenv.RecordMessage("Network initialized")

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	config := sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		State:   "network-configured",
	}

	if withShaping {
		bandwidth := runenv.SizeParam("bandwidth")
		runenv.RecordMessage("Bandwidth: %v", bandwidth)
		config.Default = sync.LinkShape{
			Bandwidth: bandwidth,
		}
	}

	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}
	runenv.RecordMessage("Network configured")

	// Get a sequence number
	runenv.RecordMessage("get a sequence number")
	seq, err := writer.Write(ctx, &sync.Subtree{
		GroupKey:    "ip-allocation",
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		},
	}, hostname)
	if err != nil {
		return err
	}

	runenv.RecordMessage("I am %d", seq)

	if seq >= 1<<16 {
		return fmt.Errorf("test-case only supports 2**16 instances")
	}

	ipC := byte((seq >> 8) + 1)
	ipD := byte(seq)

	subnet := runenv.TestSubnet.IPNet
	config.IPv4 = &subnet
	config.IPv4.IP = append(config.IPv4.IP[0:2:2], ipC, ipD)
	config.State = "ip-changed"

	runenv.RecordMessage("before writing changed ip config to redis")
	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	runenv.RecordMessage("waiting for ip-changed")
	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}
	runenv.RecordMessage("Test subnet: %v", runenv.TestSubnet)
	addrs, err := net.InterfaceAddrs()
	for _, addr := range addrs {
		runenv.RecordMessage("IP: %v", addr)
	}

	// States
	ready := sync.State("ready")

	switch {
	case seq == 1: // genesis node
		runenv.RecordMessage("Genesis: %v", config.IPv4.IP)

		runenv.RecordMessage("Pre-seal some sectors")
		cmdPreseal := exec.Command(
			"/lotus/lotus-seed",
			"pre-seal",
			"--sector-size=1024",
			"--num-sectors=2",
		)
		outfile, err := os.Create("/outputs/pre-seal.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdPreseal.Stdout = outfile
		cmdPreseal.Stderr = outfile
		err = cmdPreseal.Run()
		if err != nil {
			return err
		}

		runenv.RecordMessage("Create the genesis block and start up the first node")
		cmdNode := exec.Command(
			"/lotus/lotus",
			"daemon",
			"--lotus-make-random-genesis=/root/dev.gen",
			"--genesis-presealed-sectors=~/.genesis-sectors/pre-seal-t0101.json",
			"--bootstrap=false",
		)
		outfile, err = os.Create("/outputs/node.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdNode.Stdout = outfile
		cmdNode.Stderr = outfile
		err = cmdNode.Start()
		if err != nil {
			return err
		}

		time.Sleep(5 * time.Second)

		runenv.RecordMessage("Set up the genesis miner")
		cmdSetupMiner := exec.Command(
			"/lotus/lotus-storage-miner",
			"init",
			"--genesis-miner",
			"--actor=t0101",
			"--sector-size=1024",
			"--pre-sealed-sectors=~/.genesis-sectors",
			"--pre-sealed-metadata=~/.genesis-sectors/pre-seal-t0101.json",
			"--nosync",
		)
		outfile, err = os.Create("/outputs/miner-setup.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdSetupMiner.Stdout = outfile
		cmdSetupMiner.Stderr = outfile
		err = cmdSetupMiner.Run()
		if err != nil {
			return err
		}

		runenv.RecordMessage("Start up the miner")
		cmdMiner := exec.Command(
			"/lotus/lotus-storage-miner",
			"run",
			"--nosync",
		)
		outfile, err = os.Create("/outputs/miner.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdMiner.Stdout = outfile
		cmdMiner.Stderr = outfile
		err = cmdMiner.Start()
		if err != nil {
			return err
		}

		time.Sleep(15 * time.Second)

		// Serve /root/dev.gen file for other nodes to use as genesis
		go func() {
			http.HandleFunc("/dev.gen", func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, "/root/dev.gen")
			})

			log.Fatal(http.ListenAndServe(":9999", nil))
		}()

		genesisPeerID := "123"
		_, err = writer.Write(ctx, peerIDSubtree, &genesisPeerID)
		if err != nil {
			return err
		}

		// Signal we're ready
		_, err = writer.SignalEntry(ctx, ready)
		if err != nil {
			return err
		}
		runenv.Message("State: ready")

		/*
			// Signal we've received all the data
			_, err = writer.SignalEntry(ctx, received)
			if err != nil {
				return err
			}
		*/

		runenv.RecordSuccess()

	case seq >= 2: // additional nodes
		runenv.RecordMessage("Node: %v", seq)

		delayBetweenStarts := runenv.IntParam("delay-between-starts")
		delay := time.Duration((int(seq)-2)*delayBetweenStarts) * time.Millisecond
		runenv.RecordMessage("Delaying for %v", delay)
		time.Sleep(delay)

		// Wait until ready state is signalled.
		runenv.RecordMessage("Waiting for ready")
		err = <-watcher.Barrier(ctx, ready, 1)
		if err != nil {
			return err
		}
		runenv.RecordMessage("State: ready")

		subnet := runenv.TestSubnet.IPNet
		genesisIPv4 := &subnet
		genesisIPv4.IP = append(genesisIPv4.IP[0:2:2], 1, 1)

		// Download dev.gen file from genesis node
		resp, err := http.Get(fmt.Sprintf("http://%v:9999/dev.gen", genesisIPv4.IP))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		outfile, err := os.Create("/root/dev.gen")
		if err != nil {
			return err
		}
		io.Copy(outfile, resp.Body)

		genesisPeerIDCh := make(chan *string, 0)
		subscribeCtx, cancel := context.WithCancel(ctx)
		err = watcher.Subscribe(subscribeCtx, peerIDSubtree, genesisPeerIDCh)
		if err != nil {
			cancel()
			return err
		}
		select {
		case genesisPeerID := <-genesisPeerIDCh:
			cancel()
			runenv.RecordMessage("Genesis PeerID: %v", *genesisPeerID)
		case <-time.After(1 * time.Second):
			cancel()
			return fmt.Errorf("timeout fetching genesisPeerID")
		}
		runenv.RecordSuccess()

	default:
		return fmt.Errorf("Unexpected seq: %v", seq)
	}

	select {}

	return nil
}

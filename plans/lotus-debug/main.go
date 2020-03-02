package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/lib/jsonrpc"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	var genesisAddr string
	var genesisAddrSubtree = &sync.Subtree{
		GroupKey:    "genesisAddr",
		PayloadType: reflect.TypeOf(&genesisAddr),
		KeyFunc: func(val interface{}) string {
			return "GenesisAddr"
		},
	}

	var walletAddress string
	var walletAddressSubtree = &sync.Subtree{
		GroupKey:    "walletAddresses",
		PayloadType: reflect.TypeOf(&walletAddress),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		},
	}

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
		cmdPreseal.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		cmdNode.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		cmdSetupMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		cmdMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		api, closer, err := connectToAPI()
		if err != nil {
			return err
		}
		defer closer()

		var genesisAddrNet multiaddr.Multiaddr

		addrInfo, err := api.NetAddrsListen(ctx)
		if err != nil {
			return err
		}
		for _, addr := range addrInfo.Addrs {
			// runenv.Message("Listen addr: %v", addr.String())
			_, ip, err := manet.DialArgs(addr)
			if err != nil {
				return err
			}
			// runenv.Message("IP: %v", ip)
			// runenv.Message("Match: %v", config.IPv4.IP.String())
			if strings.Split(ip, ":")[0] == config.IPv4.IP.String() {
				genesisAddrNet = addr
			}
		}
		if genesisAddrNet == nil {
			return fmt.Errorf("Couldn't match genesis addr")
		}

		peerID, err := api.ID(ctx)
		if err != nil {
			return err
		}

		genesisAddr := fmt.Sprintf("%v/p2p/%v", genesisAddrNet.String(), peerID)
		runenv.Message("Genesis addr: %v", genesisAddr)

		_, err = writer.Write(ctx, genesisAddrSubtree, &genesisAddr)
		if err != nil {
			return err
		}

		// Signal we're ready
		_, err = writer.SignalEntry(ctx, ready)
		if err != nil {
			return err
		}
		runenv.Message("State: ready")

		localWalletAddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			cancel()
			return err
		}

		walletAddressCh := make(chan *string, 0)
		subscribeCtx, cancel := context.WithCancel(ctx)
		err = watcher.Subscribe(subscribeCtx, walletAddressSubtree, walletAddressCh)
		if err != nil {
			cancel()
			return err
		}
		for i := 1; i < runenv.TestInstanceCount; i++ {
			select {
			case walletAddress := <-walletAddressCh:
				runenv.Message("Sending funds to wallet: %v", *walletAddress)

				// Send funds - see cli/send.go in Lotus
				toAddr, err := address.NewFromString(*walletAddress)
				if err != nil {
					cancel()
					return err
				}

				val, err := types.ParseFIL("1000")
				if err != nil {
					cancel()
					return err
				}

				msg := &types.Message{
					From:     localWalletAddr,
					To:       toAddr,
					Value:    types.BigInt(val),
					GasLimit: types.NewInt(1000),
					GasPrice: types.NewInt(0),
				}

				_, err = api.MpoolPushMessage(ctx, msg)
				if err != nil {
					cancel()
					return err
				}
			}
		}
		cancel()

		runenv.RecordSuccess()

		stallAndWatchTipsetHead(ctx, runenv, api, localWalletAddr)

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

		var genesisAddrInfo *peer.AddrInfo

		genesisAddrCh := make(chan *string, 0)
		subscribeCtx, cancel := context.WithCancel(ctx)
		err = watcher.Subscribe(subscribeCtx, genesisAddrSubtree, genesisAddrCh)
		if err != nil {
			cancel()
			return err
		}
		select {
		case genesisAddr := <-genesisAddrCh:
			cancel()
			genesisMultiaddr, err := multiaddr.NewMultiaddr(*genesisAddr)
			if err != nil {
				return err
			}
			genesisAddrInfo, err = peer.AddrInfoFromP2pAddr(genesisMultiaddr)
			if err != nil {
				return err
			}
		case <-time.After(1 * time.Second):
			cancel()
			return fmt.Errorf("timeout fetching genesisAddr")
		}

		runenv.RecordMessage("Start the node")
		cmdNode := exec.Command(
			"/lotus/lotus",
			"daemon",
			"--genesis=/root/dev.gen",
			"--bootstrap=false",
		)
		cmdNode.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		api, closer, err := connectToAPI()
		if err != nil {
			return err
		}
		defer closer()

		peerID, err := api.ID(ctx)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Peer ID: %v", peerID)

		runenv.RecordMessage("Connecting to Genesis Node: %v", *genesisAddrInfo)
		err = api.NetConnect(ctx, *genesisAddrInfo)
		if err != nil {
			return err
		}

		// FIXME: Watch sync state instead
		runenv.RecordMessage("Sleeping for 15 seconds to sync")
		time.Sleep(15 * time.Second)

		runenv.RecordMessage("Creating bls wallet")
		address, err := api.WalletNew(ctx, "bls")
		if err != nil {
			return err
		}
		walletAddress := address.String()
		runenv.RecordMessage("Wallet: %v", walletAddress)

		_, err = writer.Write(ctx, walletAddressSubtree, &walletAddress)
		if err != nil {
			return err
		}

		localWalletAddr, err := api.WalletDefaultAddress(ctx)
		if err != nil {
			cancel()
			return err
		}

		count := 0
		for {
			balance, err := api.WalletBalance(ctx, address)
			if err != nil {
				return err
			}
			if balance.Sign() > 0 {
				break
			}
			time.Sleep(1 * time.Second)
			count++
			if count > 30 {
				return fmt.Errorf("Timeout waiting for funds transfer")
			}
		}

		runenv.RecordMessage("Set up the miner")
		cmdSetupMiner := exec.Command(
			"/lotus/lotus-storage-miner",
			"init",
			"--owner="+walletAddress,
		)
		cmdSetupMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		)
		cmdMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		runenv.RecordSuccess()

		stallAndWatchTipsetHead(ctx, runenv, api, localWalletAddr)

	default:
		return fmt.Errorf("Unexpected seq: %v", seq)
	}

	select {}

	return nil
}

func connectToAPI() (api.FullNode, jsonrpc.ClientCloser, error) {
	ma, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1234/http")
	if err != nil {
		return nil, nil, err
	}

	tokenContent, err := ioutil.ReadFile("/root/.lotus/token")
	if err != nil {
		return nil, nil, err
	}
	token := string(tokenContent)
	_, addr, err := manet.DialArgs(ma)
	if err != nil {
		return nil, nil, err
	}
	addr = "ws://" + addr + "/rpc/v0"

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+string(token))

	api, closer, err := client.NewFullNodeRPC(addr, headers)
	if err != nil {
		return nil, nil, err
	}

	return api, closer, nil
}

func stallAndWatchTipsetHead(ctx context.Context, runenv *runtime.RunEnv,
	api api.FullNode, address address.Address) error {
	for {
		tipset, err := api.ChainHead(ctx)
		if err != nil {
			return err
		}

		balance, err := api.WalletBalance(ctx, address)
		if err != nil {
			return err
		}

		runenv.RecordMessage("Height: %v Balance: %v", tipset.Height(), balance)
		time.Sleep(30 * time.Second)
	}
}

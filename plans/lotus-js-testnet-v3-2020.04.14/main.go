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

	"github.com/rs/cors"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
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
	genesisReadyState := sync.State("genesisReady")
	nodeReadyState := sync.State("nodeReady")
	doneState := sync.State("done")

	runenv.RecordMessage("Start up nginx")
	cmdNginx := exec.Command(
		"npm",
		"run",
		"nginx",
	)
	cmdNginx.Dir = "/plan/js-lotus-client-testground"
	outfile, err := os.Create("/outputs/nginx.out")
	if err != nil {
		return err
	}
	defer outfile.Close()
	cmdNginx.Stdout = outfile
	cmdNginx.Stderr = outfile
	err = cmdNginx.Start()
	if err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // Give nginx time to start

	mux := http.NewServeMux()

	go func() {
		handler := cors.Default().Handler(mux)
		log.Fatal(http.ListenAndServe(":9999", handler))
	}()

	// Serve token files
	mux.HandleFunc("/.lotus/token", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/root/.lotus/token")
	})

	mux.HandleFunc("/.lotusstorage/token", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/root/.lotusstorage/token")
	})

	switch {
	case seq == 1: // genesis node
		runenv.RecordMessage("Genesis: %v", config.IPv4.IP)

		if runenv.StringParam("ssh-tunnel") != "\"\"" {
			runenv.RecordMessage("Run install-ssh.sh script")
			cmdInstallSSH := exec.Command(
				"/plan/install-ssh.sh",
			)
			// cmdPreseal.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
			outfile, err := os.Create("/outputs/install-ssh.out")
			if err != nil {
				return err
			}
			defer outfile.Close()
			cmdInstallSSH.Stdout = outfile
			cmdInstallSSH.Stderr = outfile
			err = cmdInstallSSH.Run()
			if err != nil {
				return err
			}

			tunnelArgs := make([]string, 0)
			tunnelArgs = append(tunnelArgs, "-N")
			for i := 1; i <= runenv.TestInstanceCount; i++ {
				ipC := byte((i >> 8) + 1)
				ipD := byte(i)

				subnet := runenv.TestSubnet.IPNet
				nodeIPv4 := &subnet
				nodeIPv4.IP = append(config.IPv4.IP[0:2:2], ipC, ipD)
				nodeForwardArg := fmt.Sprintf(
					"RemoteForward %v %v:8001",
					11234+i-1,
					nodeIPv4.IP.String(),
				)
				tunnelArgs = append(tunnelArgs, "-o", nodeForwardArg)
				minerForwardArg := fmt.Sprintf(
					"RemoteForward %v %v:8002",
					12345+i-1,
					nodeIPv4.IP.String(),
				)
				tunnelArgs = append(tunnelArgs, "-o", minerForwardArg)
				testplanForwardArg := fmt.Sprintf(
					"RemoteForward %v %v:9999",
					30000+i-1,
					nodeIPv4.IP.String(),
				)
				tunnelArgs = append(tunnelArgs, "-o", testplanForwardArg)
			}
			tunnelArgs = append(tunnelArgs, "-o", "StrictHostKeyChecking no")
			tunnelArgs = append(tunnelArgs, runenv.StringParam("ssh-tunnel"))
			/*
				for _, arg := range tunnelArgs {
					runenv.RecordMessage("ssh arg", arg)
				}
			*/

			runenv.RecordMessage("Ssh to " + runenv.StringParam("ssh-tunnel"))
			cmdSSH := exec.Command("ssh", tunnelArgs...)
			// cmdNode.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
			outfile, err = os.Create("/outputs/ssh-tunnel.out")
			if err != nil {
				return err
			}
			defer outfile.Close()
			cmdSSH.Stdout = outfile
			cmdSSH.Stderr = outfile
			err = cmdSSH.Start()
			if err != nil {
				return err
			}
		}

		runenv.RecordMessage("Pre-seal some sectors")
		cmdPreseal := exec.Command(
			"/lotus/lotus-seed",
			"pre-seal",
			"--sector-size=2048",
			"--num-sectors=2",
		)
		// cmdPreseal.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		runenv.RecordMessage("Create localnet.json")
		cmdCreateLocalNetJSON := exec.Command(
			"/lotus/lotus-seed",
			"genesis",
			"new",
			"/root/localnet.json",
		)
		// cmdCreateLocalNetJSON.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
		outfile, err = os.Create("/outputs/create-localnet-json.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdCreateLocalNetJSON.Stdout = outfile
		cmdCreateLocalNetJSON.Stderr = outfile
		err = cmdCreateLocalNetJSON.Run()
		if err != nil {
			if runenv.BooleanParam("keep-alive") {
				runenv.RecordMessage("create localnet.json failed")
				select {}
			} else {
				return err
			}
		}

		runenv.RecordMessage("Add genesis miner")
		cmdAddGenesisMiner := exec.Command(
			"/lotus/lotus-seed",
			"genesis",
			"add-miner",
			"/root/localnet.json",
			"/root/.genesis-sectors/pre-seal-t01000.json",
		)
		// cmdAddGenesisMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
		outfile, err = os.Create("/outputs/add-genesis-miner.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdAddGenesisMiner.Stdout = outfile
		cmdAddGenesisMiner.Stderr = outfile
		err = cmdAddGenesisMiner.Run()
		if err != nil {
			return err
		}

		runenv.RecordMessage("Start up the first node")
		cmdNode := exec.Command(
			"/lotus/lotus",
			"daemon",
			"--lotus-make-genesis=/root/dev.gen",
			"--genesis-template=/root/localnet.json",
			"--bootstrap=false",
		)
		// cmdNode.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		runenv.RecordMessage("Import the genesis miner key")
		cmdImportGenesisMinerKey := exec.Command(
			"/lotus/lotus",
			"wallet",
			"import",
			"/root/.genesis-sectors/pre-seal-t01000.key",
		)
		// cmdImportGenesisMinerKey.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
		outfile, err = os.Create("/outputs/import-genesis-miner-key.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdImportGenesisMinerKey.Stdout = outfile
		cmdImportGenesisMinerKey.Stderr = outfile
		err = cmdImportGenesisMinerKey.Run()
		if err != nil {
			return err
		}

		runenv.RecordMessage("Set up the genesis miner")
		cmdSetupMiner := exec.Command(
			"/lotus/lotus-storage-miner",
			"init",
			"--genesis-miner",
			"--actor=t01000",
			"--sector-size=2048",
			"--pre-sealed-sectors=~/.genesis-sectors",
			"--pre-sealed-metadata=~/.genesis-sectors/pre-seal-t01000.json",
			"--nosync",
		)
		// cmdSetupMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		// cmdMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
		outfile, err = os.Create("/outputs/miner.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdMiner.Stdout = outfile
		cmdMiner.Stderr = outfile
		err = cmdMiner.Start()
		if err != nil {
			if runenv.BooleanParam("keep-alive") {
				runenv.RecordMessage("genesis miner failed")
				select {}
			} else {
				return err
			}
		}

		time.Sleep(15 * time.Second)

		// Serve /root/dev.gen file for other nodes to use as genesis
		mux.HandleFunc("/dev.gen", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "/root/dev.gen")
		})

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
		_, err = writer.SignalEntry(ctx, genesisReadyState)
		if err != nil {
			return err
		}
		runenv.Message("State: genesisReady")

		var localWalletAddr *address.Address
		addrs, err := api.WalletList(ctx)
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			localWalletAddr = &addr
		}
		if localWalletAddr == nil {
			return fmt.Errorf("Couldn't find wallet")
		}

		runenv.Message("Setting default wallet: %v", *localWalletAddr)
		err = api.WalletSetDefault(ctx, *localWalletAddr)
		if err != nil {
			return err
		}

		walletAddressCh := make(chan *string, 0)
		subscribeCtx, cancelSub := context.WithCancel(ctx)
		err = watcher.Subscribe(subscribeCtx, walletAddressSubtree, walletAddressCh)
		defer cancelSub()
		if err != nil {
			return err
		}
		for i := 1; i < runenv.TestInstanceCount; i++ {
			select {
			case walletAddress := <-walletAddressCh:
				runenv.Message("Sending funds to wallet: %v", *walletAddress)

				// Send funds - see cli/send.go in Lotus
				toAddr, err := address.NewFromString(*walletAddress)
				if err != nil {
					return err
				}

				val, err := types.ParseFIL("0.00001")
				if err != nil {
					return err
				}

				msg := &types.Message{
					From:     *localWalletAddr,
					To:       toAddr,
					Value:    types.BigInt(val),
					GasLimit: 1000,
					GasPrice: types.NewInt(0),
				}

				_, err = api.MpoolPushMessage(ctx, msg)
				if err != nil {
					return err
				}
			}
		}
		cancelSub()

		// Wait until nodeReady state is signalled by all secondary nodes
		runenv.RecordMessage("Waiting for nodeReady from other nodes")
		err = <-watcher.Barrier(ctx, nodeReadyState, int64(runenv.TestInstanceCount-1))
		if err != nil {
			return err
		}
		runenv.RecordMessage("State: nodeReady from other nodes")

		// Run Javascript tests from Genesis node
		runenv.RecordMessage("Run npm test")
		cmdNpmTest := exec.Command(
			"npm",
			"test",
		)
		cmdNpmTest.Dir = "/plan/js-lotus-client-testground"
		outfile, err = os.Create("/outputs/npm-test.out")
		if err != nil {
			return err
		}
		defer outfile.Close()
		cmdNpmTest.Stdout = outfile
		cmdNpmTest.Stderr = outfile
		err = cmdNpmTest.Run()
		if err != nil {
			if runenv.BooleanParam("keep-alive") {
				runenv.RecordMessage("npm test failed")
			} else {
				return err
			}
		}

		// Signal we're done and everybody should shut down
		_, err = writer.SignalEntry(ctx, doneState)
		if err != nil {
			return err
		}
		runenv.Message("State: done")

		runenv.RecordSuccess()

		if runenv.BooleanParam("keep-alive") {
			stallAndWatchTipsetHead(ctx, runenv, api, *localWalletAddr)
		}

	case seq >= 2: // additional nodes
		runenv.RecordMessage("Node: %v", seq)

		delayBetweenStarts := runenv.IntParam("delay-between-starts")
		delay := time.Duration((int(seq)-2)*delayBetweenStarts) * time.Millisecond
		runenv.RecordMessage("Delaying for %v", delay)
		time.Sleep(delay)

		// Wait until genesisReady state is signalled.
		runenv.RecordMessage("Waiting for genesisReady")
		err = <-watcher.Barrier(ctx, genesisReadyState, 1)
		if err != nil {
			return err
		}
		runenv.RecordMessage("State: genesisReady")

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
		subscribeCtx, cancelSub := context.WithCancel(ctx)
		defer cancelSub()
		err = watcher.Subscribe(subscribeCtx, genesisAddrSubtree, genesisAddrCh)
		if err != nil {
			return err
		}
		select {
		case genesisAddr := <-genesisAddrCh:
			genesisMultiaddr, err := multiaddr.NewMultiaddr(*genesisAddr)
			if err != nil {
				return err
			}
			genesisAddrInfo, err = peer.AddrInfoFromP2pAddr(genesisMultiaddr)
			if err != nil {
				return err
			}
		case <-time.After(1 * time.Second):
			return fmt.Errorf("timeout fetching genesisAddr")
		}

		runenv.RecordMessage("Start the node")
		cmdNode := exec.Command(
			"/lotus/lotus",
			"daemon",
			"--genesis=/root/dev.gen",
			"--bootstrap=false",
		)
		// cmdNode.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		address, err := api.WalletNew(ctx, wallet.ActSigType("bls"))
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
        if runenv.BooleanParam("keep-alive") {
          runenv.RecordMessage("Timeout waiting for funds transfer")
          select {}
        } else {
          return fmt.Errorf("Timeout waiting for funds transfer")
        }
			}
		}

		runenv.RecordMessage("Set up the miner")
		cmdSetupMiner := exec.Command(
			"/lotus/lotus-storage-miner",
			"init",
			"--owner="+walletAddress,
		)
		// cmdSetupMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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
		// cmdMiner.Env = append(os.Environ(), "GOLOG_LOG_LEVEL="+runenv.StringParam("log-level"))
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

		// Signal we're ready
		_, err = writer.SignalEntry(ctx, nodeReadyState)
		if err != nil {
			return err
		}
		runenv.Message("State: nodeReady")

		// Wait until done state is signalled.
		runenv.RecordMessage("Waiting for done")
		err = <-watcher.Barrier(ctx, doneState, 1)
		if err != nil {
			return err
		}
		runenv.RecordMessage("State: done")

		if runenv.BooleanParam("keep-alive") {
			stallAndWatchTipsetHead(ctx, runenv, api, localWalletAddr)
		}

	default:
		return fmt.Errorf("Unexpected seq: %v", seq)
	}

	if runenv.BooleanParam("keep-alive") {
		select {}
	}

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

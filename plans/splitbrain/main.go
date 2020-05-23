package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

type brainside int

const (
	leftbrain = iota
	rightbrain
	galaxybrain
)

func (s brainside) String() string {
	return [...]string{"leftbrain", "rightbrain", "galaxybrain"}[s]
}

type node struct {
	Side  int
	IPNet *net.IPNet
}

func main() {
	testcases := map[string]runtime.TestCaseFn{
		"drop":   shirtsAndSkins(network.Drop),
		"reject": shirtsAndSkins(network.Reject),
		"accept": shirtsAndSkins(network.Accept),
	}
	runtime.InvokeMap(testcases)
}

func setup(ctx context.Context, runenv *runtime.RunEnv) (client *sync.Client, nwclient *network.Client) {
	client = sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	if !runenv.TestSidecar {
		return nil, nil
	}

	nwclient = network.NewClient(client, runenv)
	nwclient.MustWaitNetworkInitialized(ctx)
	return client, nwclient
}

func shirtsAndSkins(action network.FilterAction) runtime.TestCaseFn {

	return func(runenv *runtime.RunEnv) error {

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		client, nwclient := setup(ctx, runenv)

		eth1, err := net.InterfaceByName("eth1")
		if err != nil {
			return err
		}
		addrs, err := eth1.Addrs()
		if err != nil {
			return err
		}

		ip, nw, err := net.ParseCIDR(addrs[0].String())
		if err != nil {
			return err
		}

		// Start an HTTP server
		runenv.RecordMessage("I have address %s", ip)
		runenv.RecordMessage("Starting http server")
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) { fmt.Fprintln(w, "hello.") })
		go http.ListenAndServe(":8765", nil)

		seq := client.MustSignalEntry(ctx, "pickaside")

		// Generate a player and publish the address to the sync service.
		me := node{int(seq) % 3, nw}
		nodeTopic := sync.NewTopic("nodes", node{})
		nodeCh := make(chan *node, 0)
		client.MustPublishSubscribe(ctx, nodeTopic, &me, nodeCh)

		// Wait until we have received all addresses
		nodes := make([]*node, 0)
		for found := 0; found < runenv.TestInstanceCount; {
			n := <-nodeCh
			nodes = append(nodes, n)
		}

		// leftbrain blocks the right brain using the prescribed method
		// leftbrain and rightbrain can't talk to each other
		// everyone can talk to galaxybrain nodes.
		if me.Side == leftbrain {
			cfg := network.Config{
				Network: "default",
			}

			for _, p := range nodes {
				if p.Side == rightbrain {
					pnet := net.IPNet{
						IP:   p.IPNet.IP,
						Mask: net.IPMask([]byte{255, 255, 255, 255}),
					}
					cfg.Rules = append(cfg.Rules, network.LinkRule{
						Subnet: pnet,
						LinkShape: network.LinkShape{
							Filter: action,
						},
					})
				}
			}
			err := nwclient.ConfigureNetwork(ctx, &cfg)
			if err != nil {
				return err
			}
		}

		// Wait until *all* nodes have received all addresses.
		client.SignalAndWait(ctx, "nodeRoundup", runenv.TestInstanceCount)
		_ = nwclient

		var errs int
		var status200 int
		var total int

		// Try to reach out to each node and see what happens.
		for _, p := range nodes {
			resp, err := http.Get(p.IPNet.IP.String() + ":8765")
			if err != nil {
				errs++
			}
			if resp.StatusCode == 200 {
				status200++
			}
			total++
		}

		runenv.RecordMessage("HTTP errors:", errs)
		runenv.RecordMessage("200 status codes", status200)
		runenv.RecordMessage("tottal", total)

		return nil
	}
}

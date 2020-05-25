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

type region int

const (
	regionA = iota
	regionB
	regionC
)

func (r region) String() string {
	return [...]string{"region_A", "region_B", "region_C"}[r]
}

type node struct {
	Region region
	IP     *net.IP
}

func main() {
	testcases := map[string]runtime.TestCaseFn{
		"drop":   routeFilter(network.Drop),
		"reject": routeFilter(network.Reject),
		"accept": routeFilter(network.Accept),
	}
	runtime.InvokeMap(testcases)
}

func expectErrors(runenv *runtime.RunEnv, a *node, b *node) bool {
	if runenv.TestCase == "accept" {
		return false
	}
	if (a.Region == regionA && b.Region == regionB) || (a.Region == regionB && b.Region == regionA) {
		return true
	}
	return false
}

func routeFilter(action network.FilterAction) runtime.TestCaseFn {

	return func(runenv *runtime.RunEnv) error {

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		client := sync.MustBoundClient(ctx, runenv)

		if !runenv.TestSidecar {
			return fmt.Errorf("this plan must be run with sidecar enabled")
		}

		netclient := network.NewClient(client, runenv)
		netclient.MustWaitNetworkInitialized(ctx)

		// Each node starts an HTTP server to test for connectivity
		runenv.RecordMessage("Starting http server")
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			runenv.RecordMessage("received http request from %s", req.RemoteAddr)
			fmt.Fprintln(w, "hello.")
		})
		go func() {
			err := http.ListenAndServe(":8765", nil)
			runenv.RecordFailure(err)
		}()

		// Race to signal this point, the sequence ID determines to which region this node belongs.
		seq := client.MustSignalEntry(ctx, "region-select")
		ip := netclient.MustGetDataNetworkIP()
		me := node{region(int(seq) % 3), &ip}
		runenv.RecordMessage("my ip is %s", ip)

		// publish my address so other nodes know how to reach me.
		nodeTopic := sync.NewTopic("nodes", node{})
		nodeCh := make(chan *node)
		_, sub := client.MustPublishSubscribe(ctx, nodeTopic, &me, nodeCh)

		// Wait until we have received all addresses
		nodes := make([]*node, 0)
		for found := 1; found <= runenv.TestInstanceCount; found++ {
			n := <-nodeCh
			if !me.IP.Equal(*n.IP) {
				nodes = append(nodes, n)
			}
		}

		// nodes from regionA apply a network policy for the nodes in regionB
		if me.Region == regionA {
			cfg := network.Config{
				Network:       "default",
				CallbackState: "reconfigured",
				Enable:        true,
			}

			for _, p := range nodes {
				if p.Region == regionB {
					pnet := net.IPNet{
						IP:   *p.IP,
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
			go netclient.MustConfigureNetwork(ctx, &cfg)
		}
		time.Sleep(10 * time.Second)

		// Wait until *all* nodes have received all addresses.
		_, err := client.SignalAndWait(ctx, "nodeRoundup", runenv.TestInstanceCount)
		if err != nil {
			return err
		}

		var unexpected error
		var errs int
		var status200 int
		var total int

		// Try to reach out to each node and see what happens.
		httpclient := http.Client{
			Timeout: time.Minute,
		}

		// When running the "accept" testcase, there should be no failures.
		// For the others, region A cannot reacon region B, so we expect failures.
		for _, p := range nodes {
			total++
			remoteAddr := "http://" + p.IP.String() + ":8765"
			runenv.RecordMessage("(region %s) contacting %s", me.Region, remoteAddr)
			resp, err := httpclient.Get(remoteAddr)
			if err != nil {
				errs++
				if !expectErrors(runenv, &me, p) {
					runenv.RecordFailure(err)
					unexpected = err
				}
				continue
			}
			if resp.StatusCode == 200 {
				status200++
			}
		}

		runenv.RecordMessage("could not connect %d", errs)
		runenv.RecordMessage("200 status codes %d", status200)
		runenv.RecordMessage("total, %d", total)

		client.Close()
		err = <-sub.Done()
		if err != nil {
			runenv.RecordFailure(err)
		}

		return unexpected
	}
}

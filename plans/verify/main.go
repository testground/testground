package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/sparrc/go-ping"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

func main() {
	testcases := map[string]runtime.TestCaseFn{
		"uses-data-network": UsesDataNetwork,
	}
	runtime.InvokeMap(testcases)
}

func setupNetwork(ctx context.Context, runenv *runtime.RunEnv) (*sync.Client, error) {
	client := sync.MustBoundClient(ctx, runenv)
	return client, client.WaitNetworkInitialized(ctx, runenv)
}

func isControlNet(nw string) bool {
	return strings.HasPrefix(nw, "192.18.") || strings.HasPrefix(nw, "100.96.")
}

// UsesDataNetwork verifies that instances can only reach each other through the data network.
// One instance acts as the target. The target publishes the IP addresses it finds to the sync
// service. Other instances will subscribe to the topic and test for network connectivity to the
// target on each of its ip addresses.
// An error is reported if the target is reachable over the control network or if there is packet
// loss over the data network.
func UsesDataNetwork(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := setupNetwork(ctx, runenv)
	if err != nil {
		runenv.RecordFailure(err)
		return err
	}
	defer client.Close()

	const (
		_ int64 = iota
		targetmode
		pingmode
	)

	endOfNetworks := "endOfNetworks"

	netTopic := sync.NewTopic("addrs", "")

	switch client.MustSignalAndWait(ctx, "ready", runenv.TestInstanceCount) {
	case targetmode:
		runenv.RecordMessage("target mode. publishing target networks.")
		for _, iname := range []string{"eth0", "eth1"} {
			iface, err := net.InterfaceByName(iname)
			if err != nil {
				runenv.RecordFailure(err)
				return err
			}
			addrs, err := iface.Addrs()
			if err != nil {
				runenv.RecordFailure(err)
				return err
			}
			for _, addr := range addrs {
				runenv.RecordMessage("publishing %s", addr)
				_, _ = client.Publish(ctx, netTopic, addr.String())
			}
		}
		_, _ = client.Publish(ctx, netTopic, endOfNetworks)
		runenv.RecordMessage("published my addresses from all networks to sync service. ready to be tested.")
		_, _ = client.SignalEntry(ctx, "target-ready")

	case pingmode:
		runenv.RecordMessage("ping mode. waiting for target networks.")
		<-client.MustBarrier(ctx, "target-ready", 1).C
		runenv.RecordMessage("starting ping")
		nwCh := make(chan string)
		_, _ = client.Subscribe(ctx, netTopic, nwCh)
		for network := <-nwCh; network != endOfNetworks; network = <-nwCh {
			runenv.RecordMessage("checking if network is reachable: %s", network)
			addr := strings.Split(network, "/")[0]
			pinger, _ := ping.NewPinger(addr)
			pinger.Count = 10
			pinger.Interval = 500 * time.Millisecond
			pinger.Timeout = time.Second
			pinger.SetPrivileged(true) // Use ICMP ping rather than UDP ping. Root in container.
			pinger.OnFinish = func(stat *ping.Statistics) {
				// If we are pinging the control network, expect no response, else, expect a response.
				if isControlNet(addr) && stat.PacketLoss != 100.0 {
					runenv.RecordFailure(errors.New("error - control network is accessible; it should not be"))
				} else if !isControlNet(addr) && stat.PacketLoss > 0.0 {
					runenv.RecordFailure(errors.New("error - data network is not accessible; it should be"))
				}
				runenv.RecordMessage("packet loss on %s: %f%%", network, stat.PacketLoss)
			}
			pinger.Run()
		}
	}

	_ = client.MustSignalAndWait(ctx, "finished", runenv.TestInstanceCount)

	return nil
}

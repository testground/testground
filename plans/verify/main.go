package main

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/sparrc/go-ping"
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

	// Race to get a sequence ID.
	// Seq 1 will be the target for pings, the other will be the instance who performs the pings
	race := sync.NewTopic("race", "")
	seq, err := client.Publish(ctx, race, runenv.TestRun)
	if err != nil {
		return err
	}

	addrTopic := sync.NewTopic("addrs", &net.IPAddr{})

	switch seq {
	case targetmode:
		runenv.RecordMessage("target mode")

		eth0, err := net.InterfaceByName("eth0")
		if err != nil {
			runenv.RecordFailure(err)
			return err
		}
		eth1, err := net.InterfaceByName("eth1")
		if err != nil {
			runenv.RecordFailure(err)
			return err
		}

		// Get IP addresses from eth0 and eth1 and publish my IP address
		for _, iface := range []*net.Interface{eth0, eth1} {
			addrs, err := iface.Addrs()
			if err != nil {
				runenv.RecordFailure(err)
				return err
			}
			for _, addr := range addrs {
				runenv.RecordMessage("publishing %s", addr)
				client.Publish(ctx, addrTopic, &addr)
			}
		}

		runenv.RecordMessage("Ready to be pinged.")
		client.SignalEntry(ctx, "target-ready")
		<-client.MustBarrier(ctx, "pinger-finished", 1).C

	case pingmode:
		runenv.RecordMessage("pingmode")
		<-client.MustBarrier(ctx, "target-ready", 1).C
		runenv.RecordMessage("starting ping")
		adCh := make(chan *net.Addr)
		client.Subscribe(ctx, addrTopic, adCh)
		for addrp := range adCh {
			addr := *addrp
			runenv.RecordMessage("pinging %s", addr)
			pinger, _ := ping.NewPinger(strings.Split(addr.Network(), "/")[0])
			pinger.Count = 3
			pinger.Interval = time.Second
			pinger.OnFinish = func(stat *ping.Statistics) {
				runenv.RecordMessage("addr: %s -- sent: %d, recvd: %d", stat.Addr, stat.PacketsSent, stat.PacketsRecv)
			}
			pinger.Run()
		}
		client.SignalEntry(ctx, "pinger-finished")
	}

	return nil
}

package main

import (
	"context"
	"net"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

func ExampleDropNetwork(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	runenv.RecordMessage("before sync.MustBoundClient")
	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	if !runenv.TestSidecar {
		return nil
	}

	netclient := network.NewClient(client, runenv)
	runenv.RecordMessage("before netclient.MustWaitNetworkInitialized")
	netclient.MustWaitNetworkInitialized(ctx)

	// Add a rule which drops outbound traffic to 1.2.3.4
	// Of course, replace this IP address with
	config := &network.Config{
		Network: "default",
		Enable:  true,
		Default: network.LinkShape{
			Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		},
		Rules: []network.LinkRule{
			{
				LinkShape: network.LinkShape{
					Filter: network.Drop,
				},
				Subnet: net.IPNet{
					IP:   net.IPv4(1, 2, 3, 4),
					Mask: net.IPMask([]byte{255, 255, 255, 255}),
				},
			},
		},
		CallbackState: "network-configured",
	}

	runenv.RecordMessage("before netclient.MustConfigureNetwork")
	netclient.MustConfigureNetwork(ctx, config)
	runenv.RecordMessage("after netclient.MustConfigureNetwork")

	// This will give you time to log in and see what's going on.
	time.Sleep(10 * time.Minute)

	return nil
}

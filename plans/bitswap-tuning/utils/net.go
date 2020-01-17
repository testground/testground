package utils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
func SetupNetwork(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) error {
	if !runenv.TestSidecar {
		return nil
	}

	// Wait for the network to be initialized.
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	// TODO: just put the hostname inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	latency := time.Duration(runenv.IntParam("latency_ms")) * time.Millisecond
	bandwidth := runenv.IntParam("bandwidth_mb")
	writer.Write(sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   latency,
			Bandwidth: uint64(bandwidth) * 1024 * 1024,
		},
		State: "network-configured",
	})

	err = <-watcher.Barrier(ctx, "network-configured", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

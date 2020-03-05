package network

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

const (
	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check cluster_k8s.go for more information
	NetworkInitialisationSuccessful = "network initialisation successful"
	NetworkInitialisationFailed     = "network initialisation failed"
)

// WaitNetworkInitialized waits for the sidecar to initialize the network, if the sidecar is enabled.
func WaitNetworkInitialized(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher) error {
	if runenv.TestSidecar {
		err := <-watcher.Barrier(ctx, "network-initialized", int64(runenv.TestInstanceCount))
		if err != nil {
			runenv.RecordMessage(NetworkInitialisationFailed)
			return fmt.Errorf("failed to initialize network: %w", err)
		}
	}
	runenv.RecordMessage(NetworkInitialisationSuccessful)
	return nil
}

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case. The configuration for this function relies on a few test params, which should be
// provided by the relevant toml file. When successful, this method returns the applied bandwidth
// and enters a configured state for the applied network.
//
// toml configuration:
//  bandwidth_mb
//  latency_ms
func SetupNetwork(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, network string) (time.Duration, int, error) {
	if !runenv.TestSidecar {
		return 0, 0, nil
	}
	// Wait for the network to be initialized.
	if err := WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return 0, 0, err
	}
	// TODO: just put the unique testplan id inside the runenv?
	hostname, err := os.Hostname()
	if err != nil {
		return 0, 0, err
	}
	latency := time.Duration(runenv.IntParam("latency_ms")) * time.Millisecond
	if err != nil {
		return 0, 0, err
	}
	bandwidth := runenv.IntParam("bandwidth_mb")
	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: network,
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   latency,
			Bandwidth: uint64(bandwidth) * 1024 * 1024,
		},
		State: sync.State(fmt.Sprintf("%s-configurd", network)),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to configure network: %w", err)
	}
	runenv.RecordMessage("Configured Network latency: %d bandwidth: %d", latency, bandwidth)

	return latency, bandwidth, nil
}

// Configure the network and wait for all instances to to complete the network setup also.
func SetupNetworkWaitAll(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, network string) (time.Duration, int, error) {
	latency, bandwidth, err := SetupNetwork(ctx, runenv, watcher, writer, network)
	if err != nil {
		return latency, bandwidth, err
	}
	err = <-watcher.Barrier(ctx, "network-configured", int64(runenv.TestInstanceCount))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to configure network: %w", err)
	}
	return latency, bandwidth, nil
}

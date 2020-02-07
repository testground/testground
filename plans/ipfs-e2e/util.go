package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// GetComms is a convenience method for getting a communication channel between tests.
// Pass a context, a string which is used as the key for the subtree, and your runenv.
// returns a Watcher, writer, your sequence number, and any possible errors.
func GetComms(ctx context.Context, key string, runenv *runtime.RunEnv) (*sync.Watcher, *sync.Writer, int64, error) {

	watcher, writer := sync.MustWatcherWriter(runenv)

	runenv.Message("Waiting for network initialization")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return watcher, writer, -1, err
	}
	runenv.Message("Network initilization complete")

	st := sync.Subtree{
		GroupKey:    key,
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		}}
	seq, err := writer.Write(&st, runenv.TestRun)
	runenv.Message("I have sequence ID %d\n", seq)
	return watcher, writer, seq, err
}

// SetupNetwork instructs the sidecar (if enabled) to setup the network for this
// test case.
// This method borrows heavily from the bitswap-tuning plan of the same name.
// TODO: create some method like this in the plans SDK?
func SetupNetwork(ctx context.Context, runenv *runtime.RunEnv, latency_ms int, bandwidth_mb int) error {
	if !runenv.TestSidecar {
		return nil
	}

	watcher, writer := sync.MustWatcherWriter(runenv)

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	writer.Write(sync.NetworkSubtree(hostname), &sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		Default: sync.LinkShape{
			Latency:   time.Duration(latency_ms) * time.Millisecond,
			Bandwidth: uint64(bandwidth_mb) * 1024 * 1024,
		},
		State: "network-configured",
	})

	err = <-watcher.Barrier(ctx, "network-configured", int64(runenv.TestInstanceCount))
	if err != nil {
		return fmt.Errorf("failed to configure network: %w", err)
	}
	return nil
}

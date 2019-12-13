package sync

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

// WaitNetworkInitialized waits for the sidecar to initialize the network, if the sidecar is enabled.
func WaitNetworkInitialized(ctx context.Context, runenv *runtime.RunEnv, watcher *Watcher) error {
	if runenv.TestSidecar {
		err := <-watcher.Barrier(ctx, "network-initialized", int64(runenv.TestInstanceCount))
		if err != nil {
			return fmt.Errorf("failed to initialize network: %w", err)
		}
	}
	return nil
}

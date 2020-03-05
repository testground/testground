package sync

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

// Depreciated. Use sdk.network.WaitNetworkInitialized
// TODO: remove this once this plans have migrated
const (
	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check cluster_k8s.go for more information
	NetworkInitialisationSuccessful = "network initialisation successful"
	NetworkInitialisationFailed     = "network initialisation failed"
)

// WaitNetworkInitialized waits for the sidecar to initialize the network, if the sidecar is enabled.
func WaitNetworkInitialized(ctx context.Context, runenv *runtime.RunEnv, watcher *Watcher) error {
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

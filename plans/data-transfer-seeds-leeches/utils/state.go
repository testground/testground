package utils

import (
	"context"

	"github.com/ipfs/testground/sdk/sync"
)

func SignalAndWaitForAll(ctx context.Context, instanceCount int, stateName string, watcher *sync.Watcher, writer *sync.Writer) error {
	// Set a state barrier.
	state := sync.State(stateName)
	doneCh := watcher.Barrier(ctx, state, int64(instanceCount))

	// Signal we're done on the end state.
	_, err := writer.SignalEntry(state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	if err = <-doneCh; err != nil {
		return err
	}

	return nil
}

package utils

import (
	"context"

	"github.com/ipfs/testground/sdk/sync"
)

func SignalAndWaitForAll(ctx context.Context, instanceCount int, stateName string, watcher *sync.Watcher, writer *sync.Writer) error {
	// Set a state barrier.
	state := sync.State(stateName)
	doneCh := watcher.Barrier(ctx, state, int64(instanceCount))

	// Signal we've entered the state.
	_, err := writer.SignalEntry(ctx, state)
	if err != nil {
		return err
	}

	// Wait until all others have signalled.
	return <-doneCh
}

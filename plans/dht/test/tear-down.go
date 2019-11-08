package test

import (
	"context"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// TearDown creates the Barrier state
func TearDown(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer) {
	// Set a state barrier.
	end := sync.State("end")
	doneCh := watcher.Barrier(ctx, end, int64(runenv.TestInstanceCount))

	// Signal we're done on the end state.
	_, err := writer.SignalEntry(end)
	if err != nil {
		runenv.Abort(err)
		return
	}

	// Wait until all others have signalled.
	if err := <-doneCh; err != nil {
		runenv.Abort(err)
		return
	}
}

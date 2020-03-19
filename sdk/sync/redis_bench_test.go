package sync

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func BenchmarkBarrier(b *testing.B) {
	b.ReportAllocs()

	close := ensureRedis(b)
	defer close()

	runenv := randomRunEnv()

	for n := 0; n < b.N; n++ {
		state := State(fmt.Sprintf("yoda-%d", n))

		// simulate 1000 instances, each signalling entry, and waiting for all others
		total := 1000

		var wg sync.WaitGroup
		for i := 0; i < total; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				watcher, writer := MustWatcherWriter(context.Background(), runenv)
				defer watcher.Close()
				defer writer.Close()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				writer.SignalEntry(ctx, state)
				<-watcher.Barrier(ctx, state, int64(total))
			}()
		}
		wg.Wait()
	}
}

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

	watcher, writer := MustWatcherWriter(context.Background(), runenv)
	defer watcher.Close()
	defer writer.Close()

	target := 1000000
	workers := 10
	each := target / workers

	for n := 0; n < b.N; n++ {
		ctx, cancel := context.WithCancel(context.Background())
		state := State(fmt.Sprintf("yoda-%d", n))

		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()

				for i := start; i < end; i++ {
					_, _ = writer.SignalEntry(ctx, state)
				}
			}(i*each, (i+1)*each)
		}

		b.ResetTimer()
		ch := watcher.Barrier(ctx, state, int64(target))
		<-ch
		cancel()
	}
}

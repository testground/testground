package main

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func signalForever(ctx context.Context, runenv *runtime.RunEnv, writer *sync.Writer) {
	buf := make([]byte, 10)
	_, _ = rand.Reader.Read(buf)
	writer.SignalEntry(ctx, sync.State(string(buf)))
	for {
		writer.SignalEntry(ctx, sync.State(buf))
		time.Sleep(time.Duration(500) * time.Millisecond)
	}
}

// This function exercises SignalEntry with different number of concurrent writers
// Spin up more and more goprocesses, each of which signals a small value.
// Meanwhile, the main gothread also publishes a small value and publishes metrics
func signalConcurrencyBench(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, writer := sync.MustWatcherWriter(ctx, runenv)

	concurrentMetric := runtime.MetricDefinition{
		Name:           "concurrency",
		Unit:           "concurrent go-funcs",
		ImprovementDir: 1,
	}

	timeMetric := runtime.MetricDefinition{
		Name:           "Sync Time",
		Unit:           "nanoseconds",
		ImprovementDir: -1,
	}

	numbad := 0
	numconcurrent := 0
	maxnanoseconds := int64(0)
	minnanoseconds := int64(1<<63 - 1) // A large number for 64-bit signed ints
	for i := 0; numbad <= 5; i += (1024 * 1024) {
		go signalForever(ctx, runenv, writer)
		numconcurrent += 1
		start := time.Now().UnixNano()
		writer.SignalEntry(ctx, sync.State("testtesttest"))
		end := time.Now().UnixNano()
		nanoseconds := end - start
		if nanoseconds >= 5*1000*1000*1000 { // 5 seconds
			numbad += 1
		}
		maxnanoseconds = max(maxnanoseconds, nanoseconds)
		minnanoseconds = min(minnanoseconds, nanoseconds)
		runenv.RecordMetric(&concurrentMetric, float64(numconcurrent))
		runenv.RecordMetric(&timeMetric, float64(nanoseconds))
	}
	runenv.RecordMessage("Ending benchmark. max concurrency: %d, max time: %d, min time: %d", numconcurrent, maxnanoseconds, minnanoseconds)
	return nil
}

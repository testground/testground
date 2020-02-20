package main

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func min(x, y int64) int64 {
	if x > y {
		return y
	}
	return x
}

// This function exercises SignalEntry with different sizes
// Step up the size of the state 1MB at a time and record the time it takes to publish
// the state into the redis backend.
func signalSizeBench(runenv *runtime.RunEnv) error {
	ctx := context.Background()

	_, writer := sync.MustWatcherWriter(ctx, runenv)

	sizeMetric := runtime.MetricDefinition{
		Name:           "State Size",
		Unit:           "bytes",
		ImprovementDir: 1,
	}

	timeMetric := runtime.MetricDefinition{
		Name:           "Sync Time",
		Unit:           "nanoseconds",
		ImprovementDir: -1,
	}

	throughputMetric := runtime.MetricDefinition{
		Name:           "Sync Throughput",
		Unit:           "bytes/sec",
		ImprovementDir: 1,
	}

	r := rand.Reader
	numbad := 0
	maxnanoseconds := int64(0)
	minnanoseconds := int64(1<<63 - 1) // A large number for 64-bit signed ints
	maxsize := int64(0)
	for i := 0; numbad <= 5; i += (1024 * 1024) {
		buf := make([]byte, i)
		num, err := r.Read(buf)
		if err != nil {
			runenv.RecordFailure(err)
		}
		if num != i {
			runenv.RecordMessage("Read %d out of requested %d random bytes", num, i)
		}
		start := time.Now().UnixNano()
		writer.SignalEntry(ctx, sync.State(string(buf)))
		end := time.Now().UnixNano()
		nanoseconds := end - start
		if nanoseconds >= 5*1000*1000*1000 { // 5 seconds
			numbad += 1
		}
		maxsize = max(maxsize, int64(num))
		maxnanoseconds = max(maxnanoseconds, nanoseconds)
		minnanoseconds = min(minnanoseconds, nanoseconds)
		runenv.RecordMetric(&sizeMetric, float64(num))
		runenv.RecordMetric(&timeMetric, float64(nanoseconds))
		runenv.RecordMetric(&throughputMetric, 1000000000*float64(num)/float64(nanoseconds))
	}
	runenv.RecordMessage("Ending benchmark. Largest state size: %d, max time: %d, min time: %d", maxsize, maxnanoseconds, minnanoseconds)
	return nil
}

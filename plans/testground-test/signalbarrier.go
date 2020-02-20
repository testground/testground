package main

import (
	"context"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// This function is intended to exercise the barier time
// This makes sense only when there is a lot of nodes.
func signalBarrierBench(runenv *runtime.RunEnv) error {
	ctx := context.Background()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	iterations := runenv.IntParam("iterations")

	barrierMetric := runtime.MetricDefinition{
		Name:           "Barrier Latency",
		Unit:           "nanoseconds",
		ImprovementDir: -1,
	}

	var total int64
	for i := 1; i <= iterations; i += 1 {
		ping := sync.State("ping")
		pong := sync.State("pong")
		start := time.Now().UnixNano()
		writer.SignalEntry(ctx, ping)
		_ = <-watcher.Barrier(ctx, ping, int64(i*runenv.TestInstanceCount))
		end := time.Now().UnixNano()
		writer.SignalEntry(ctx, pong)
		nanoseconds := end - start
		total = nanoseconds
		runenv.RecordMetric(&barrierMetric, float64(nanoseconds))
	}
	runenv.RecordMessage("Average barrier latency, %d", int(total)/iterations)
	return nil
}

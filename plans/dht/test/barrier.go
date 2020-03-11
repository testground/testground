package test

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func BarrierTest(runenv *runtime.RunEnv) error {
	opts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	node, _, err := Setup(ctx, runenv, watcher, writer, opts)
	if err != nil {
		return err
	}

	defer Teardown(ctx, runenv, watcher, writer)

	if err := testBarrier(ctx, runenv, watcher, writer, node); err != nil {
		return err
	}
	return nil
}

func testBarrier(ctx context.Context, runenv *runtime.RunEnv, watcher *sync.Watcher, writer *sync.Writer, node *NodeParams) error {
	stager := NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "barrier", watcher, writer, runenv)

	for i := 0; i < 100; i++ {
		stager.Begin()
		t := time.Now()
		err := stager.End()
		if err != nil {
			return err
		}
		runenv.RecordMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("stage-time"),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Since(t).Nanoseconds()))
	}
	return nil
}

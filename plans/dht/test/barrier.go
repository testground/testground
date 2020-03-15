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

	ri := &RunInfo{
		runenv:  runenv,
		watcher: watcher,
		writer:  writer,
	}

	node, _, err := Setup(ctx, ri, opts)
	if err != nil {
		return err
	}

	//defer Teardown(ctx, ri)

	if err := testSync(ctx, ri, node); err != nil {
		//Teardown(ctx, ri)
		return err
	}

	//Teardown(ctx, ri)

	return nil
}

func testBarrier(ctx context.Context, ri *RunInfo, node *NodeParams) error {
	stager := NewBatchStager(ctx, node.info.Seq, ri.runenv.TestInstanceCount, "barrier", ri)

	for i := 0; i < 100; i++ {
		stager.Begin()
		t := time.Now()
		err := stager.End()
		if err != nil {
			return err
		}
		ri.runenv.RecordMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("stage-time %d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Since(t).Nanoseconds()))
	}
	return nil
}

func testSync(ctx context.Context, ri *RunInfo, node *NodeParams) error {
	for i := 0; i < 100; i++ {
		t := time.Now()
		if err := Sync(ctx, ri, sync.State(fmt.Sprintf("synctest %d",i))) ; err != nil {
			panic(err)
		}
		ri.runenv.RecordMetric(&runtime.MetricDefinition{
			Name:           fmt.Sprintf("stage-time: %d", i),
			Unit:           "ns",
			ImprovementDir: -1,
		}, float64(time.Since(t).Nanoseconds()))
	}
	return nil
}

package test

import (
	"context"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func BootstrapNetwork(runenv *runtime.RunEnv) error {
	commonOpts := GetCommonOpts(runenv)

	ctx, cancel := context.WithTimeout(context.Background(), commonOpts.Timeout)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	//defer watcher.Close()
	//defer writer.Close()

	ri := &RunInfo{
		runenv:  runenv,
		watcher: watcher,
		writer:  writer,
	}

	node, peers, err := Setup(ctx, ri, commonOpts)
	if err != nil {
		return err
	}

	defer outputGraph(node.dht, "end")
	defer Teardown(ctx, ri)

	stager := NewBatchStager(ctx, node.info.Seq, runenv.TestInstanceCount, "default", ri)

	// Bring the network into a nice, stable, bootstrapped state.
	if err = Bootstrap(ctx, ri, commonOpts, node, peers, stager, GetBootstrapNodes(commonOpts, node, peers)); err != nil {
		return err
	}

	if commonOpts.RandomWalk {
		if err = RandomWalk(ctx, runenv, node.dht); err != nil {
			return err
		}
	}

	return nil
}

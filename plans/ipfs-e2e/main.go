package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/testground/plans/ipfs-e2e/test"
	"github.com/ipfs/testground/plans/ipfs-e2e/util"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func main() {
	runtime.Invoke(run)
}
func run(runenv *runtime.RunEnv) error {
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	watcher, writer, seq, err := util.GetComms(ctx, "main", runenv)
	defer watcher.Close()
	defer writer.Close()
	if err != nil {
		return err
	}

	tc := test.TestCase{}
	// Odd sequence numbers are "old"
	// Even sequence numers are "new"
	// the first two are "seed" nodes.
	// Also, some of the nodes will be considered "slow"
	// Depending on the sequence number, assign different options.
	if seq%2 == 0 {
		// use old DHT and Bitswap
	} else {
		// Use new DHT and Bitswap
	}

	if seq < 3 {
		tc.Role = test.Seeder
	} else {
		tc.Role = test.Leecher
	}

	if seq%3 == 0 {
		// edit the networking to make it slower
	}

	nodeOpts := iptb.NodeOpts{
		Initialize: true,
		Start:      true,
	}

	spec := iptb.NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(nodeOpts)
	ensemble := iptb.NewTestEnsemble(ctx, spec)
	defer ensemble.Destroy()

	// start test for each file size.
	MB := 1024 * 1024

	for size := MB; size <= 128*MB; size *= 2 {
		waitmsg := fmt.Sprintf("main: test size %d", size)
		waiting := sync.State(waitmsg)
		runenv.Message(waitmsg)
		writer.SignalEntry(waiting)
		err := <-watcher.Barrier(ctx, waiting, int64(runenv.TestInstanceCount))
		if err != nil {
			return err
		}
		writer.SignalEntry("running")
		tc.FileSize = size
		tc.Execute(runenv, ensemble, ctx)
	}
	return nil
}

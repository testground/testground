package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	//ctx, _ := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	watcher, writer, _, err := GetComms(ctx, "main", runenv)
	// These don't need to be closed because the deferred cancel a few lines up already closes them.
	defer watcher.Close()
	defer writer.Close()
	if err != nil {
		return err
	}

	nodeOpts := iptb.NodeOpts{
		Initialize: true,
		Start:      true,
	}

	spec := iptb.NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(nodeOpts)
	ensemble := iptb.NewTestEnsemble(ctx, spec)
	ensemble.Initialize()
	defer ensemble.Destroy()

	testConfig := IptbTestConfig{Ensemble: ensemble}
	var testCase IptbTestCase

	if strings.Contains(runenv.TestGroupID, "seeders") {
		testCase = SeedingTestCase{&testConfig}
		SetupNetwork(ctx, runenv, 100, 100)
	} else if strings.Contains(runenv.TestGroupID, "leechers") {
		testCase = SeedingTestCase{&testConfig}
		SetupNetwork(ctx, runenv, 200, 75)
	} else {
		return errors.New("passive nodes are not implemented.")
	}

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
		runenv.Message("now running")
		testConfig.FileName = "filename"
		testConfig.FileSize = size
		testCase.Execute(runenv)
	}
	runenv.Message("all done")
	return nil
}

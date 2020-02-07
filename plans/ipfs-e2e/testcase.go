package main

import (
	"context"
	_ "errors"
	_ "fmt"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

const (
	Setup        = sync.State("setup")
	Ready        = sync.State("ready")
	Seeding      = sync.State("seeding")
	SeedingDone  = sync.State("seeding done")
	Leeching     = sync.State("leeching")
	LeechingDone = sync.State("leeching done")
)

/*
IptbTestCase has a single Execute method which can be used by runtime.Invoke
*/
type IptbTestCase interface {
	Execute(*runtime.RunEnv) error
}

type IptbTestConfig struct {
	Ensemble *iptb.TestEnsemble
	FileSize int
	FileName string
}

/*
SeedingTestCase Implements IptbTestCase for seeding ipfs nodes
1. Add randomized file to repo
2. Transition to Ready
3. Wait until all hosts are ready.
4. Transition to Seeding.
5. Wait until all leechers transition into LeechingDone.
6. Transition to SeedingDone
7. Emit metric
8. Terminate
*/
type SeedingTestCase struct {
	Config *IptbTestConfig
}

func (tc SeedingTestCase) Execute(runenv *runtime.RunEnv) error {
	ctx := context.Background()
	watcher, writer, _, err := GetComms(ctx, "testcase", runenv)
	defer watcher.Close()
	defer writer.Close()
	if err != nil {
		return err
	}
	writer.SignalEntry(Setup)
	runenv.Message("Step 1: Adding file.")
	hash, err := tc.AddRandomFile(runenv)
	runenv.Message("Added hash %s", hash)

	runenv.Message("Step 2: Transition to Ready")
	writer.SignalEntry(Ready)

	runenv.Message("Step 3: Waiting for all to be ready")
	_ = <-watcher.Barrier(ctx, Ready, int64(runenv.TestInstanceCount))

	runenv.Message("Step 4: Transition to Seeding")
	writer.SignalEntry(Seeding)
	before := time.Now().UnixNano()

	runenv.Message("Step 5: Wait until all leechers transition to LeechingDone")
	_ = <-watcher.Barrier(ctx, LeechingDone, int64(runenv.TestInstanceCount))
	after := time.Now().UnixNano()

	runenv.Message("Step 6: Transition to SeedingDone")
	writer.SignalEntry(SeedingDone)

	runenv.Message("Step 7: Emit Metric")
	seedTime := after - before
	metric := runtime.MetricDefinition{
		Name:           "seed time",
		Unit:           "Nanoseconds",
		ImprovementDir: -1,
	}
	runenv.EmitMetric(&metric, float64(seedTime))

	runenv.Message("Step 8: Goodbye.")
	return nil
}

// this method is mostly copied from smlbench simple_add.go
func (tc SeedingTestCase) AddRandomFile(runenv *runtime.RunEnv) (string, error) {
	ensemble := tc.Config.Ensemble
	node := ensemble.GetNode("seeder")
	client := node.Client()

	filePath, err := runenv.CreateRandomFile(ensemble.TempDir(), int64(tc.Config.FileSize))
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer os.Remove(filePath)

	hash, err := client.Add(file)
	if err != nil {
		return "", err
	}
	return hash, nil
}

/*
LeechingTestCase implements IptbTest case for leeching ipfs nodes
1. Transition to Ready
2. Wait until at least two nodes have transitioned to Seeding
3. Transition to Leeching
4. Leech file.
5. Transition to LeechingDone
6. Emit metric
7. Terminate
*/
type LeechingTestCase struct {
	Config *IptbTestConfig
}

func (tc LeechingTestCase) Execute(runenv *runtime.RunEnv) error {
	ctx := context.Background()
	watcher, writer, _, err := GetComms(ctx, "testcase", runenv)
	defer watcher.Close()
	defer writer.Close()
	if err != nil {
		return err
	}
	writer.SignalEntry(Setup)
	time.Sleep(time.Duration(30) * time.Second)
	writer.SignalEntry(Leeching)
	time.Sleep(time.Duration(30) * time.Second)
	writer.SignalEntry(LeechingDone)

	return nil
}

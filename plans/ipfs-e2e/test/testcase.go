package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/testground/plans/ipfs-e2e/util"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type TestRole int

const (
	Seeder TestRole = iota
	Leecher
	Passive
)

func (r TestRole) String() string {
	return [...]string{"Seeder", "Leecher"}[r]
}

type TestCase struct {
	Role     TestRole
	FileSize int
}

// This test runs for each file size tested.
// 1. Leechers enter the "not leaching" state.
// 2. Seeders wait for all leachers to enter the "not leaching" state.
// 3. Now that all the leachers are ready, seeders transition to the "seeding" state
// 4. The leechers wait until both seeders have transitioned to seeding before they proceed with
// leeching.
// You might be wondering... why not just start leeching as soon as one seeder is available? I want to
// prevent a race condition where one seeder starts, and the other fails to start because there are
// not enough leechers still in the "not seeding" state".
// 5. After the leechers have successfully downloaded the file, they transition to "done leeching",
// then terminate
// 6. Seeders wait until all leachers have reached the "done leeching" state, emit a metric, and
// terminate.
func (tc *TestCase) Execute(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble, ctx context.Context) error {
	watcher, writer, _, err := util.GetComms(ctx, "main", runenv)
	defer watcher.Close()
	defer writer.Close()
	if err != nil {
		return err
	}
	seeding := sync.State("seeding")
	doneseeding := sync.State("done seeding")

	notleeching := sync.State("not leeching")
	leeching := sync.State("leeching")
	doneleeching := sync.State("done leeching")

	expectedSeeders := 2
	expectedLeechers := runenv.TestGroupInstanceCount - expectedSeeders

	switch tc.Role {
	case Seeder:
		defer func() { writer.SignalEntry(doneseeding) }()
		err = <-watcher.Barrier(ctx, notleeching, int64(expectedLeechers))
		if err != nil {
			return err
		}
		writer.SignalEntry(seeding)
		before := time.Now()
		err := seedFiles(runenv, ensemble)
		if err != nil {
			return err
		}
		after := time.Now()
		metric := runtime.MetricDefinition{
			Name:           "seed time",
			Unit:           "Nanoseconds",
			ImprovementDir: -1,
		}

		runenv.EmitMetric(&metric, float64(after.UnixNano()-before.UnixNano()))
		err = <-watcher.Barrier(ctx, doneleeching, int64(expectedLeechers))
		if err != nil {
			return err
		}
		return nil
	case Leecher:
		defer func() { writer.SignalEntry(doneleeching) }()
		writer.SignalEntry(notleeching)
		err := <-watcher.Barrier(ctx, seeding, int64(expectedSeeders))
		if err != nil {
			return err
		}
		writer.SignalEntry(leeching)
		return leechFiles(runenv, ensemble)
	default:
		return errors.New(fmt.Sprintf("test case role is not defined or not implemented %s", tc.Role))
	}
}

func seedFiles(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble) error {
	return nil
}

func leechFiles(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble) error {
	return nil
}

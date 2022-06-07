package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

// ExampleSync demonstrates synchronization between instances in the test group.
//
// In this example, the first instance to signal enrollment becomes the leader
// of the test case.
//
// The leader waits until all the followers have reached the state "ready"
// then, the followers wait for a signal from the leader to be released.
func ExampleSync(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		enrolledState = sync.State("enrolled")
		readyState    = sync.State("ready")
		releasedState = sync.State("released")

		ctx = context.Background()
	)

	// retrieve the sync service client, it has been initialized by the SDK.
	client := initCtx.SyncClient

	// signal entry in the 'enrolled' state, and obtain a sequence number.
	seq := client.MustSignalEntry(ctx, enrolledState)

	runenv.RecordMessage("my sequence ID: %d", seq)

	// if we're the first instance to signal, we'll become the LEADER.
	if seq == 1 {
		runenv.RecordMessage("i'm the leader.")
		numFollowers := runenv.TestInstanceCount - 1

		// let's wait for the followers to signal.
		runenv.RecordMessage("waiting for %d instances to become ready", numFollowers)
		err := <-client.MustBarrier(ctx, readyState, numFollowers).C
		if err != nil {
			return err
		}

		runenv.RecordMessage("the followers are all ready")
		runenv.RecordMessage("ready...")
		time.Sleep(1 * time.Second)
		runenv.RecordMessage("set...")
		time.Sleep(5 * time.Second)
		runenv.RecordMessage("go, release followers!")

		// signal on the 'released' state.
		client.MustSignalEntry(ctx, releasedState)
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	sleep := rand.Intn(5)
	runenv.RecordMessage("i'm a follower; signalling ready after %d seconds", sleep)
	time.Sleep(time.Duration(sleep) * time.Second)
	runenv.RecordMessage("follower signalling now")

	// signal entry in the 'ready' state.
	client.MustSignalEntry(ctx, readyState)

	// wait until the leader releases us.
	err := <-client.MustBarrier(ctx, releasedState, 1).C
	if err != nil {
		return err
	}

	runenv.RecordMessage("i have been released")
	return nil
}

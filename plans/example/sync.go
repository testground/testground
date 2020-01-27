package main

import (
	"context"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"math/rand"
	"reflect"
	"time"
)

// ExampleSync demonstrates synchronization between instances in the test group.
// The backend for synchronization is a redis queue, but this detail is abstracted
// away from us as the Watcher.
// In this example, the first instance to write becomes the leader of the test.
// The leader waits until all the followers have reached the state "ready"
// then, the followers wait for a signal from the leader to start.
func ExampleSync(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	runenv.Message("Waiting for network initialization")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}
	runenv.Message("Network initilization complete")

	st := sync.Subtree{
		GroupKey:    "messages",
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		}}

	seq, err := writer.Write(&st, runenv.TestRun)
	if err != nil {
		return err
	}

	runenv.Message("My seqeuence ID: %d\n", seq)

	readyState := sync.State("ready")
	startState := sync.State("start")

	if seq == 1 {
		runenv.Message("I'm the boss.")
		numFollowers := runenv.TestInstanceCount - 1
		runenv.Message("Waiting for %d instances to become ready", numFollowers)
		err := <-watcher.Barrier(ctx, readyState, int64(numFollowers))
		if err != nil {
			return err
		}
		runenv.Message("The followers are all ready")
		runenv.Message("Ready...")
		time.Sleep(1 * time.Second)
		runenv.Message("Set...")
		time.Sleep(5 * time.Second)
		runenv.Message("Go!")
		writer.SignalEntry(startState)
		return nil
	} else {

		rand.Seed(time.Now().UnixNano())
		sleepTime := rand.Intn(10)
		runenv.Message("I'm a follower. Signaling ready after %d seconds", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
		writer.SignalEntry(readyState)
		err = <-watcher.Barrier(ctx, startState, 1)
		if err != nil {
			return err
		}
		runenv.Message("Received Start")
		return nil
	}
}

package main

import (
	"context"
	"math/rand"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
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
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	runenv.RecordMessage("Waiting for network initialization")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}
	runenv.RecordMessage("Network initilization complete")

	st := sync.Subtree{
		GroupKey:    "messages",
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		}}

	seq, err := writer.Write(ctx, &st, runenv.TestRun)
	if err != nil {
		return err
	}

	runenv.RecordMessage("My sequence ID: %d", seq)

	readyState := sync.State("ready")
	startState := sync.State("start")

	if seq == 1 {
		runenv.RecordMessage("I'm the boss.")
		numFollowers := runenv.TestInstanceCount - 1
		runenv.RecordMessage("Waiting for %d instances to become ready", numFollowers)
		err := <-watcher.Barrier(ctx, readyState, int64(numFollowers))
		if err != nil {
			return err
		}
		runenv.RecordMessage("The followers are all ready")
		runenv.RecordMessage("Ready...")
		time.Sleep(1 * time.Second)
		runenv.RecordMessage("Set...")
		time.Sleep(5 * time.Second)
		runenv.RecordMessage("Go!")
		_, err = writer.SignalEntry(ctx, startState)
		return err
	} else {

		rand.Seed(time.Now().UnixNano())
		sleepTime := rand.Intn(10)
		runenv.RecordMessage("I'm a follower. Signaling ready after %d seconds", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
		_, err = writer.SignalEntry(ctx, readyState)
		if err != nil {
			return err
		}
		err = <-watcher.Barrier(ctx, startState, 1)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Received Start")
		return nil
	}
}

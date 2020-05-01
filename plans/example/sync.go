package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

// ExampleSync demonstrates synchronization between instances in the test group.
// The backend for synchronization is a redis queue, but this detail is abstracted
// away from us as the Watcher.
//
// In this example, the first instance to write becomes the leader of the test.
// The leader waits until all the followers have reached the state "ready"
// then, the followers wait for a signal from the leader to start.
func ExampleSync(runenv *runtime.RunEnv) error {
	var (
		readyState = sync.State("ready")
		startState = sync.State("start")
	)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	netclient := network.NewClient(client, runenv)
	runenv.RecordMessage("Waiting for network initialization")

	netclient.MustWaitNetworkInitialized(ctx)
	runenv.RecordMessage("Network initilization complete")

	topic := sync.NewTopic("messages", "")

	seq, err := client.Publish(ctx, topic, runenv.TestRun)
	if err != nil {
		return err
	}

	runenv.RecordMessage("My sequence ID: %d", seq)

	if seq == 1 {
		runenv.RecordMessage("I'm the boss.")
		numFollowers := runenv.TestInstanceCount - 1

		runenv.RecordMessage("Waiting for %d instances to become ready", numFollowers)
		err := <-client.MustBarrier(ctx, readyState, numFollowers).C
		if err != nil {
			return err
		}

		runenv.RecordMessage("The followers are all ready")
		runenv.RecordMessage("Ready...")
		time.Sleep(1 * time.Second)
		runenv.RecordMessage("Set...")
		time.Sleep(5 * time.Second)
		runenv.RecordMessage("Go!")

		client.MustSignalEntry(ctx, startState)
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	sleepTime := rand.Intn(10)
	runenv.RecordMessage("I'm a follower. Signaling ready after %d seconds", sleepTime)
	time.Sleep(time.Duration(sleepTime) * time.Second)

	client.MustSignalEntry(ctx, readyState)

	err = <-client.MustBarrier(ctx, startState, 1).C
	if err != nil {
		return err
	}

	runenv.RecordMessage("Received Start")
	return nil
}

package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

// StartTimeBench does nothing but start up and report the time it took to start.
// This relies on the testground daemon to inject the time when the plan is scheduled
// into the runtime environment
func StartTimeBench(runenv *runtime.RunEnv) error {
	elapsed := time.Since(runenv.TestStartTime)
	runenv.R().RecordPoint("time_to_start_secs", elapsed.Seconds())
	return nil
}

// NetworkInitBench starts and waits for the network to initialize
// The metric it emits represents the time between plan start and when the network initialization
// is completed.
func NetworkInitBench(runenv *runtime.RunEnv) error {
	// FIX(cory/raulk) this test will yield a false zero value on local:exec,
	// because it doesn't support the sidecar yet. We should probably skip it
	// conditionally, based on the runner. We might want to inject the runner
	// name in the runenv, so tests like this can modify their behaviour
	// accordingly.
	startupTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	netclient := network.NewClient(client, runenv)
	netclient.MustWaitNetworkInitialized(ctx)

	elapsed := time.Since(startupTime)
	runenv.R().RecordPoint("time_to_network_init_secs", elapsed.Seconds())
	return nil
}

// NetworkLinkShapeBench benchmarks the time required to change the link shape
func NetworkLinkShapeBench(runenv *runtime.RunEnv) error {
	// FIX(cory/raulk) this test will not work with local:exec, because it
	// doesn't support the sidecar yet. We should probably skip it
	// conditionally, based on the runner. We might want to inject the runner
	// name in the runenv, so tests like this can modify their behaviour
	// accordingly.

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	netclient := network.NewClient(client, runenv)
	netclient.MustWaitNetworkInitialized(ctx)

	// A new network configuration
	cfg := &network.Config{
		Network: "default",
		Default: network.LinkShape{
			Latency: 250 * time.Millisecond,
		},
		CallbackState:  sync.State(fmt.Sprintf("callback-%d", rand.Int63())),
		CallbackTarget: 1,
	}

	before := time.Now()

	// Send configuration to the sidecar.
	netclient.MustConfigureNetwork(ctx, cfg)

	elapsed := time.Since(before)
	runenv.R().RecordPoint("time_to_shape_network_secs", elapsed.Seconds())

	return nil
}

// BarrierBench tests the time it takes to wait on Barriers, waiting on a
// different number of instances in each loop.
func BarrierBench(runenv *runtime.RunEnv) error {
	iterations := runenv.IntParam("barrier_iterations")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(runenv.IntParam("barrier_test_timeout_secs"))*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	netclient := network.NewClient(client, runenv)
	netclient.MustWaitNetworkInitialized(ctx)

	type cfg struct {
		Name    string
		Timer   runtime.Timer
		Percent float64
	}

	var tests []*cfg
	for percent := 0.2; percent <= 1.0; percent += 0.2 {
		name := fmt.Sprintf("barrier_time_%d_percent", int(percent*100))

		t := cfg{
			Name:    name,
			Timer:   runenv.R().Timer(name),
			Percent: percent,
		}
		tests = append(tests, &t)
	}

	// Loop a bunch of times to generate a good amount of data.
	// The number of loops is pretty arbitrary here.
	for i := 1; i <= iterations; i++ {
		for _, tst := range tests {
			readyState := sync.State(fmt.Sprintf("ready_%d_%s", i, tst.Name))
			testState := sync.State(fmt.Sprintf("test_%d_%s", i, tst.Name))
			testInstanceNum := int(math.Floor(float64(runenv.TestInstanceCount) * tst.Percent))

			if testInstanceNum == 0.0 {
				testInstanceNum = 1.0
			}

			client.MustSignalAndWait(ctx, readyState, runenv.TestInstanceCount)

			barrierTestStart := time.Now()
			client.MustSignalAndWait(ctx, testState, testInstanceNum)
			elapsed := time.Since(barrierTestStart)

			runenv.R().RecordPoint(tst.Name, elapsed.Seconds())

			tst.Timer.Update(elapsed)
		}
	}

	return nil
}

// SubtreeBench benchmarks publish and subsciptions to a subtree
func SubtreeBench(runenv *runtime.RunEnv) error {
	rand.Seed(time.Now().UnixNano())

	iterations := runenv.IntParam("subtree_iterations")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(runenv.IntParam("subtree_test_timeout_secs"))*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	netclient := network.NewClient(client, runenv)
	netclient.MustWaitNetworkInitialized(ctx)

	topic := sync.NewTopic("instances", "")

	seq, err := client.Publish(ctx, topic, &runenv.TestRun)
	if err != nil {
		return err
	}

	mode := "receive"
	if seq == 1 {
		mode = "publish"
	}

	type testSpec struct {
		Metric string
		Data   *string
		Topic  *sync.Topic
		Timer  runtime.Timer
	}

	// Create tests ranging from 64B to 4KiB.
	// Note: anything over 1500 is likely to have ethernet fragmentation.
	var tests []*testSpec
	for size := 64; size <= 4*1024; size = size << 1 {
		name := fmt.Sprintf("subtree_time_%d_bytes", size)
		d := make([]byte, 0, size)
		rand.Read(d)
		data := string(d)

		metric := name + "_" + mode

		ts := &testSpec{
			Metric: metric,
			Data:   &data,
			Topic:  sync.NewTopic(name, ""),
			Timer:  runenv.R().Timer(metric),
		}
		tests = append(tests, ts)
	}

	var (
		handoff = sync.State("handoff")
		end     = sync.State("end")
	)

	switch mode {
	case "publish":
		runenv.RecordMessage("i am the publisher")

		for _, tst := range tests {
			for i := 1; i <= iterations; i++ {
				t := time.Now()
				_, err = client.Publish(ctx, tst.Topic, tst.Data)
				if err != nil {
					return err
				}
				tst.Timer.UpdateSince(t)
				runenv.R().RecordPoint(tst.Metric+"_secs", time.Since(t).Seconds())

				if i%500 == 0 {
					runenv.RecordMessage("published %d items (series: %s)", i, tst.Metric)
				}
			}
		}
		// signal to subscribers they can start.
		_, err = client.SignalEntry(ctx, handoff)
		if err != nil {
			return err
		}

		// wait for everyone to be done; this is necessary because the sync
		// service applies TTL by electing the first 5 publishers on the Redis
		// Stream to own the keepalive. In this case, all 5 publishers will be
		// THE publisher. In an ordinary test case, each instance will write a
		// key and therefore the ownership will be distributed. That does not
		// happen here, as all key publishing is concentrated on the publisher.
		client.MustSignalAndWait(ctx, end, runenv.TestGroupInstanceCount)

	case "receive":
		defer func() {
			_, err := client.SignalEntry(ctx, end)
			if err != nil {
				panic(err)
			}
		}()

		runenv.RecordMessage("i am a subscriber")

		// if we are receiving, wait for the publisher to be done.
		<-client.MustBarrier(ctx, handoff, 1).C

		for _, tst := range tests {
			ch := make(chan *string, 1)
			_, err = client.Subscribe(ctx, tst.Topic, ch)
			if err != nil {
				return err
			}
			for i := 1; i <= iterations; i++ {
				t := time.Now()
				b := <-ch
				tst.Timer.UpdateSince(t)

				runenv.R().RecordPoint(tst.Metric+"_secs", time.Since(t).Seconds())

				if strings.Compare(*b, *tst.Data) != 0 {
					return fmt.Errorf("received unexpected value")
				}
				if i%500 == 0 {
					runenv.RecordMessage("received %d items (series: %s)", i, tst.Metric)
				}
			}
		}
	}

	return nil
}

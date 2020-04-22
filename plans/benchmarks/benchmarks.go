package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/testground/testground/sdk/runtime"
	"github.com/testground/testground/sdk/sync"
)

// This method emits the time as output. It does *not* emit a prometheus metric.
func emitTime(runenv *runtime.RunEnv, name string, duration time.Duration) {
	runenv.RecordMetric(&runtime.MetricDefinition{
		Name:           name,
		Unit:           "seconds",
		ImprovementDir: -1,
	}, duration.Seconds())
}

// StartTimeBench does nothing but start up and report the time it took to start.
// This relies on the testground daemon to inject the time when the plan is scheduled
// into the runtime environment
func StartTimeBench(runenv *runtime.RunEnv) error {
	elapsed := time.Since(runenv.TestStartTime)
	emitTime(runenv, "Time to start", elapsed)

	gauge := runenv.M().NewGauge(runtime.GaugeOpts{Name: "start_time", Help: "time from plan scheduled to plan booted"})
	gauge.Set(float64(elapsed))
	return nil
}

// NetworkInitBench starts and waits for the network to initialize
// The metric it emits represents the time between plan start and when the network initialization
// is completed.
func NetworkInitBench(runenv *runtime.RunEnv) error {
	// FIX(cory/raulk) this test will not work with local:exec, because it
	// doesn't support the sidecar yet. We should probably skip it
	// conditionally, based on the runner. We might want to inject the runner
	// name in the runenv, so tests like this can modify their behaviour
	// accordingly.
	startupTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	if err := client.WaitNetworkInitialized(ctx, runenv); err != nil {
		return err
	}

	elapsed := time.Since(startupTime)
	emitTime(runenv, "Time to network init", elapsed)

	gauge := runenv.M().NewGauge(runtime.GaugeOpts{Name: "net_init_time", Help: "Time waiting for network initialization"})
	gauge.Set(float64(elapsed))
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

	if err := client.WaitNetworkInitialized(ctx, runenv); err != nil {
		return err
	}

	// A state name unique to the container...
	//
	// FIX(cory/raulk): this name is not unique in local:exec; it will be the
	// host's name.
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	doneState := sync.State("net configured " + name)

	// A new network configuration
	netConfig := sync.NetworkConfig{
		Network: "default",
		Default: sync.LinkShape{
			Latency: 250 * time.Millisecond,
		},
		State: doneState,
	}

	beforeNetConfig := time.Now()

	// Send configuration to the sidecar.
	_, err = client.Publish(ctx, sync.NetworkTopic(name), &netConfig)
	if err != nil {
		return err
	}
	// Wait for the signal that the network change is completed.
	err = <-client.MustBarrier(ctx, doneState, 1).C
	if err != nil {
		return err
	}
	duration := time.Since(beforeNetConfig)
	emitTime(runenv, "Time to configure link shape", duration)
	gauge := runenv.M().NewGauge(runtime.GaugeOpts{Name: "link_shape_time", Help: "time waiting for change in network link shape"})
	gauge.Set(float64(duration))

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

	if err := client.WaitNetworkInitialized(ctx, runenv); err != nil {
		return err
	}

	type cfg struct {
		Name    string
		Gauge   prometheus.Gauge
		Percent float64
	}

	var tests []*cfg
	for percent := 0.2; percent <= 1.0; percent += 0.2 {
		name := fmt.Sprintf("barrier_time_%d_percent", int(percent*100))
		t := cfg{
			Name:    name,
			Gauge:   runenv.M().NewGauge(runtime.GaugeOpts{Name: name, Help: fmt.Sprintf("time waiting for %f barrier", percent)}),
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

			_, err := client.SignalEntry(ctx, readyState)
			if err != nil {
				return err
			}

			<-client.MustBarrier(ctx, readyState, runenv.TestInstanceCount).C

			barrierTestStart := time.Now()
			_, err = client.SignalEntry(ctx, testState)
			if err != nil {
				return err
			}
			<-client.MustBarrier(ctx, sync.State(testState), testInstanceNum).C

			duration := time.Since(barrierTestStart)
			emitTime(runenv, tst.Name, duration)

			// I picked `Add` here instead of `Set` so the measurement will have to be rated.
			// The reason I did this is so the rate will drop to zero after the end of the test
			// Use rate(barrier_time_XX_percent) in prometheus graphs.
			tst.Gauge.Add(float64(duration))
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

	if err := client.WaitNetworkInitialized(ctx, runenv); err != nil {
		return err
	}

	topic := sync.NewTopic("instances","")

	seq, err := client.Publish(ctx, topic, &runenv.TestRun)
	if err != nil {
		return err
	}

	mode := "receive"
	if seq == 1 {
		mode = "publish"
	}

	type testSpec struct {
		Name    string
		Data    *string
		Topic   *sync.Topic
		Summary runtime.Summary
	}

	// Create tests ranging from 64B to 4KiB.
	// Note: anything over 1500 is likely to have ethernet fragmentation.
	var tests []*testSpec
	for size := 64; size <= 4*1024; size = size << 1 {
		name := fmt.Sprintf("subtree_time_%d_bytes", size)
		d := make([]byte, 0, size)
		rand.Read(d)
		data := string(d)

		ts := &testSpec{
			Name: name,
			Data: &data,
			Topic: sync.NewTopic(name, ""),
			Summary: runenv.M().NewSummary(runtime.SummaryOpts{
				Name:       name + "_" + mode,
				Help:       fmt.Sprintf("time to %s %d bytes", mode, size),
				Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.001, 0.99: 0.001},
			}),
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
				t := prometheus.NewTimer(tst.Summary)
				_, err = client.Publish(ctx, tst.Topic, tst.Data)
				if err != nil {
					return err
				}
				t.ObserveDuration()

				if i%500 == 0 {
					runenv.RecordMessage("published %d items (series: %s)", i, tst.Name)
				}
			}
		}
		// signal to subscribers they can start.
		_, err = client.SignalEntry(ctx, handoff)
		if err != nil {
			return err
		}

		_, err = client.SignalEntry(ctx, end)
		if err != nil {
			return err
		}

		// wait for everyone to be done; this is necessary because the sync
		// service applies TTL by electing the first 5 publishers on the Redis
		// Stream to own the keepalive. In this case, all 5 publishers will be
		// THE publisher. In an ordinary test case, each instance will write a
		// key and therefore the ownership will be distributed. That does not
		// happen here, as all key publishing is concentrated on the publisher.
		<-client.MustBarrier(ctx, end, runenv.TestGroupInstanceCount).C

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
				t := prometheus.NewTimer(tst.Summary)
				b := <-ch
				t.ObserveDuration()
				if strings.Compare(*b, *tst.Data) != 0 {
					return fmt.Errorf("received unexpected value")
				}
				if i%500 == 0 {
					runenv.RecordMessage("received %d items (series: %s)", i, tst.Name)
				}
			}
		}
	}

	return nil
}

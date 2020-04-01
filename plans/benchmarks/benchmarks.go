package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/prometheus/client_golang/prometheus"
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

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
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

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
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
	_, err = writer.Write(ctx, sync.NetworkSubtree(name), &netConfig)
	if err != nil {
		return err
	}
	// Wait for the signal that the network change is completed.
	err = <-watcher.Barrier(ctx, doneState, 1)
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
	iterations := runenv.IntParam("iterations")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
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
			testInstanceNum := int64(math.Floor(float64(runenv.TestInstanceCount) * tst.Percent))

			if testInstanceNum == 0.0 {
				testInstanceNum = 1.0
			}

			_, err := writer.SignalEntry(ctx, readyState)
			if err != nil {
				return err
			}

			<-watcher.Barrier(ctx, readyState, int64(runenv.TestInstanceCount))

			barrierTestStart := time.Now()
			_, err = writer.SignalEntry(ctx, testState)
			if err != nil {
				return err
			}
			<-watcher.Barrier(ctx, sync.State(testState), testInstanceNum)

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

	iterations := runenv.IntParam("iterations")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	st := &sync.Subtree{
		GroupKey:    "instances",
		PayloadType: reflect.TypeOf((*string)(nil)),
		KeyFunc: func(val interface{}) string {
			return string(*val.(*string))
		},
	}

	seq, err := writer.Write(ctx, st, &runenv.TestRun)
	if err != nil {
		return err
	}

	mode := "receive"
	if seq == 1 {
		mode = "publish"
	}

	type testSpec struct {
		Name    string
		Data    []byte
		Subtree *sync.Subtree
		Summary runtime.Summary
	}

	// Create tests ranging from 64B to 64KiB.
	// Note: anything over 1500 is likely to have ethernet fragmentation.
	var tests []*testSpec
	for size := 64; size <= 64*1024; size = size << 1 {
		name := fmt.Sprintf("subtree_time_%s_%d_bytes", mode, size)
		desc := fmt.Sprintf("time to %s %d bytes", mode, size)
		data := make([]byte, 0, size)
		rand.Read(data)

		ts := &testSpec{
			Name: name,
			Data: data,
			Subtree: &sync.Subtree{
				GroupKey:    name,
				PayloadType: reflect.TypeOf(data),
			},
			Summary: runenv.M().NewSummary(runtime.SummaryOpts{
				Name:       name,
				Help:       desc,
				Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.001, 0.99: 0.001},
			}),
		}
		tests = append(tests, ts)
	}

	handoff := sync.State("handoff")

	switch mode {
	case "publish":
		runenv.RecordMessage("i am the publisher")

		for j, tst := range tests {
			for i := 1; i <= iterations; i++ {
				if i%1000 == 0 {
					runenv.RecordMessage(fmt.Sprintf("publisher on test case %d iteration %d", j, i))
				}
				t := prometheus.NewTimer(tst.Summary)
				_, err = writer.Write(ctx, tst.Subtree, tst.Data)
				if err != nil {
					return err
				}
				t.ObserveDuration()
			}
		}
		// signal to subscribers they can start.
		runenv.RecordMessage("signal entry to handoff")
		_, err = writer.SignalEntry(ctx, handoff)
		if err != nil {
			return err
		}

	case "receive":
		runenv.RecordMessage("i am a subscriber")

		// if we are receiving, wait for the publisher to be done.
		<-watcher.Barrier(ctx, handoff, int64(1))

		for j, tst := range tests {
			runenv.RecordMessage(fmt.Sprintf("subscriber on test case %d", j))
			ch := make(chan []byte, 1)
			err = watcher.Subscribe(ctx, tst.Subtree, ch)
			if err != nil {
				return err
			}
			for i := 1; i <= iterations; i++ {
				if i%1000 == 0 {
					runenv.RecordMessage(fmt.Sprintf("subscriber on test case %d, iteration %d", j, i))
				}
				t := prometheus.NewTimer(tst.Summary)
				b := <-ch
				t.ObserveDuration()
				if !bytes.Equal(tst.Data, b) {
					return fmt.Errorf("received unexpected value")
				}
			}
		}
	}

	return nil
}

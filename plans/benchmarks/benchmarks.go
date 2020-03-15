package main

import (
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

type subtreeBenchCfg struct {
	Name  string
	Gauge prometheus.Gauge
	Data  []byte
}

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

	gauge := runtime.NewGauge(runenv, "start_time", "time from plan scheduled to plan booted")
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

	gauge := runtime.NewGauge(runenv, "net_init_time", "Time waiting for network initialization")
	gauge.Set(float64(elapsed))
	return nil
}

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
	gauge := runtime.NewGauge(runenv, "link_shape_time", "time waiting for change in network link shape")
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
			Gauge:   runtime.NewGauge(runenv, name, fmt.Sprintf("time waiting for %f barrier", percent)),
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

func SubtreePublishBench(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	var msg []byte
	st := sync.Subtree{
		GroupKey:    "messages",
		PayloadType: reflect.TypeOf(&msg),
		KeyFunc: func(val interface{}) string {
			return string(*val.(*[]byte))
		}}

	seq, err := writer.Write(ctx, &st, runenv.TestRun)
	if err != nil {
		return err
	}

	// Create several tests up to 1MB
	// Anything over 1500 is likely to have ethernet fragmentation
	// Do these sizes make any sense at all? I really don't know.
	var tests []*subtreeBenchCfg
	for size := 64; size <= 1024*1024; size = size << 1 {
		buf := make([]byte, size)
		name := fmt.Sprintf("subtree_bench_%s_bytes", string(size))
		t := subtreeBenchCfg{
			Name:  name,
			Gauge: runtime.NewGauge(runenv, name, fmt.Sprintf("Time to transmmit %d bytes", size)),
			Data:  buf,
		}
		tests = append(tests, &t)

	}

	if seq == 1 {
		// Publish information to the subtree and wait for all to receive it.
		return subtreePublisher(ctx, runenv, writer, watcher, &st, tests)
	} else {
		// Subscribe to the subtree as quickly as quickly as possible.
		return subtreeSubscriber(ctx, runenv, writer, watcher, &st, tests)
	}
}

func subtreePublisher(ctx context.Context, runenv *runtime.RunEnv, writer *sync.Writer, watcher *sync.Watcher, st *sync.Subtree, tests []*subtreeBenchCfg) error {

	rand.Seed(time.Now().UnixNano())

	// The sender doesn't publish metrics.
	// Just keep everyone on the same page, and publsh fresh data.
	// Wait until all are ready,
	// publish
	// begin the test
	// Repeat.

	for i := 0; i <= 100000; i++ {
		for _, tst := range tests {
			readyState := sync.State(fmt.Sprintf("ready_%d_%s", i, tst.Name))
			testState := sync.State(fmt.Sprintf("test_%d_%s", i, tst.Name))
			rand.Read(tst.Data)
			_, err := writer.SignalEntry(ctx, readyState)
			if err != nil {
				return err
			}
			<-watcher.Barrier(ctx, readyState, int64(runenv.TestInstanceCount))
			writer.SignalEntry(ctx, testState)
			writer.Write(ctx, st, tst.Data)
		}
	}
	return nil
}

// Subscribers signal they are ready, and then wait until there is one test signal.
// Once they see that signal, measure the time between the test signal entry and data
// is retrieved from the subscription channel.
func subtreeSubscriber(ctx context.Context, runenv *runtime.RunEnv, writer *sync.Writer, watcher *sync.Watcher, st *sync.Subtree, tests []*subtreeBenchCfg) error {

	for i := 0; i <= 100000; i++ {
		for _, tst := range tests {
			readyState := sync.State(fmt.Sprintf("ready_%d_%s", i, tst.Name))
			testState := sync.State(fmt.Sprintf("test_%d_%s", i, tst.Name))
			writer.SignalEntry(ctx, readyState)
			_ = <-watcher.Barrier(ctx, testState, int64(1))
			ch := make(chan []byte, 1)
			timeStart := time.Now()
			watcher.Subscribe(ctx, st, ch)
			<-ch
			tst.Gauge.Set(float64(time.Since(timeStart).Milliseconds()))
		}
	}
	return nil
}

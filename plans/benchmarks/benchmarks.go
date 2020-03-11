package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
	"github.com/prometheus/client_golang/prometheus"
)

type barrierBenchCfg struct {
	Name    string
	Gauge   prometheus.Gauge
	Percent float64
}

// This method emits the time as output. It does *not* emit a prometheus metric.
func emitTime(runenv *runtime.RunEnv, name string, duration time.Duration) {
	runenv.RecordMetric(&runtime.MetricDefinition{
		Name:           name,
		Unit:           "Seconds",
		ImprovementDir: -1,
	},
		duration.Seconds())
}

// StartTimeBench does nothing but start up and report the time it took to start.
// This relies on the testground daemon to inject the time when the plan is scheduled
// into the runtime environment
func StartTimeBench(runenv *runtime.RunEnv) error {
	scheduledTime := runenv.TestStartTime
	duration := time.Since(scheduledTime)
	emitTime(runenv, "Time to Start", duration)
	gauge := runtime.NewGauge(runenv, "start_time", "time from plan scheduled to plan booted")
	gauge.Set(float64(duration))
	return nil
}

// NetworkInitBench starts and waits for the network to initialize
// The metric it emits represents the time between plan start and when the network initialization
// is completed.
func NetworkInitBench(runenv *runtime.RunEnv) error {
	startupTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	duration := time.Since(startupTime)
	emitTime(runenv, "Time to Network", duration)
	gauge := runtime.NewGauge(runenv, "net_init_time", "Time waiting for network initialization")
	gauge.Set(float64(duration))
	return nil
}

func NetworkLinkShapeBench(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}
	// A state name unique to the container...
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

// BarrierBench tests the time it takes to wait on Barriers, waiting on a different number
// of instances in each loop.
func BarrierBench(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	var tests []*barrierBenchCfg

	for percent := 0.2; percent <= 1.0; percent += 0.2 {
		name := fmt.Sprintf("barrier_time_%d_percent", int(percent*100))
		t := barrierBenchCfg{
			Name:    name,
			Gauge:   runtime.NewGauge(runenv, name, fmt.Sprintf("time waiting for %f barrier", percent)),
			Percent: percent,
		}
		tests = append(tests, &t)
	}

	// Loop a bunch of times to generate a good amount of data.
	// The number of loops is pretty arbitrary here.
	for i := 0; i <= 100000; i++ {
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

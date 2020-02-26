package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

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
	startupTime := time.Now()
	emitTime(runenv, "Time to Start", startupTime.Sub(scheduledTime))
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

	emitTime(runenv, "Time to Network", time.Now().Sub(startupTime))
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
	writer.Write(ctx, sync.NetworkSubtree(name), &netConfig)
	// Wait for the signal that the network change is completed.
	err = <-watcher.Barrier(ctx, doneState, 1)
	if err != nil {
		return err
	}
	emitTime(runenv, "Time to configure link shape", time.Now().Sub(beforeNetConfig))
	return nil
}

func NetworkIpChangeBench(runenv *runtime.RunEnv) error {
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
		IPv4:    &runenv.TestSubnet.IPNet,
		State:   doneState,
	}
	// Change the IP address.
	// Not checking if the IP address I'm changing to already exists, by the way
	ipBytes := []byte(netConfig.IPv4.IP)
	ipBytes[3] = ipBytes[3] ^ byte(255)
	netConfig.IPv4.IP = net.IP(ipBytes)

	beforeNetConfig := time.Now()
	// Send configuration to the sidecar.
	writer.Write(ctx, sync.NetworkSubtree(name), &netConfig)
	// Wait for the signal that the network change is completed.
	err = <-watcher.Barrier(ctx, doneState, 1)
	if err != nil {
		return err
	}
	emitTime(runenv, "Time to change IP address", time.Now().Sub(beforeNetConfig))
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

	for percent := 0.2; percent <= 1.0; percent += 0.2 {
		readyState := sync.State(fmt.Sprintf("barrier test ready %f", percent))
		testInstanceNum := int64(math.Floor(float64(runenv.TestInstanceCount) * percent))
		if testInstanceNum == 0.0 {
			testInstanceNum = 1.0
		}
		testLoopName := fmt.Sprintf("barrier test for %d instances (%d%%)", testInstanceNum, int(100*percent))
		testState := sync.State(testLoopName)
		writer.SignalEntry(ctx, readyState)
		<-watcher.Barrier(ctx, readyState, int64(runenv.TestInstanceCount))
		barrierTestStart := time.Now()
		writer.SignalEntry(ctx, testState)
		<-watcher.Barrier(ctx, sync.State(testState), testInstanceNum)
		emitTime(runenv, testLoopName, time.Now().Sub(barrierTestStart))
	}

	return nil
}

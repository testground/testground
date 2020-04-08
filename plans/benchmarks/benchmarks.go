package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/influxdata/influxdb-client-go"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func metricsWriter(runenv *runtime.RunEnv) influxdb2.WriteApi {
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}
	opts := influxdb2.DefaultOptions()
	opts.SetTlsConfig(&tlsConfig)

	client := influxdb2.NewClientWithOptions(runenv.TestInfluxURL, runenv.TestInfluxToken, opts)
	return client.WriteApi(runenv.TestInfluxOrg, runenv.TestInfluxBucket)
}

func makePoint(runenv *runtime.RunEnv, measurement string, fields map[string]interface{}) *influxdb2.Point {
	tags := map[string]string{
		"TestCase":   runenv.TestCase,
		"TestPlan":   runenv.TestPlan,
		"TestRun":    runenv.TestRun,
		"TestCommit": runenv.TestCommit,
		"TestBranch": runenv.TestBranch,
		"TestRepo":   runenv.TestRepo,
	}
	return influxdb2.NewPoint(measurement, tags, fields, time.Now())
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

	w := metricsWriter(runenv)
	w.WritePoint(makePoint(runenv, "time to start", map[string]interface{}{"start_time": elapsed}))
	w.Flush()

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

	w := metricsWriter(runenv)
	defer w.Flush()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	elapsed := time.Since(startupTime)
	emitTime(runenv, "Time to network init", elapsed)

	p := makePoint(runenv, "time waiting for network initialization",
		map[string]interface{}{"net_init_time": float64(elapsed)})
	w.WritePoint(p)
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

	w := metricsWriter(runenv)
	defer w.Flush()

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

	p := makePoint(runenv, "time waiting for change in network link shape",
		map[string]interface{}{"link_shape_time": float64(duration)})
	w.WritePoint(p)

	return nil
}

// BarrierBench tests the time it takes to wait on Barriers, waiting on a
// different number of instances in each loop.
func BarrierBench(runenv *runtime.RunEnv) error {
	iterations := runenv.IntParam("barrier_iterations")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(runenv.IntParam("barrier_test_timeout_secs"))*time.Second)
	defer cancel()

	w := metricsWriter(runenv)
	defer w.Flush()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	type cfg struct {
		Name    string
		Percent float64
	}

	var tests []*cfg
	for percent := 0.2; percent <= 1.0; percent += 0.2 {
		name := fmt.Sprintf("barrier_time_%d_percent", int(percent*100))
		t := cfg{
			Name:    name,
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
			p := makePoint(runenv, "time spent waitng for barrier",
				map[string]interface{}{
					"barrier_percent": tst.Percent,
					"duration":        duration,
				})
			w.WritePoint(p)
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

	w := metricsWriter(runenv)
	defer w.Flush()

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
		Data    *string
		Subtree *sync.Subtree
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
			Subtree: &sync.Subtree{
				GroupKey:    name,
				PayloadType: reflect.TypeOf((*string)(nil)),
			},
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
				before := time.Now()
				_, err = writer.Write(ctx, tst.Subtree, tst.Data)
				if err != nil {
					return err
				}
				duration := time.Since(before)
				p := makePoint(runenv, "subtree publish time",
					map[string]interface{}{
						"size":         len(*tst.Data),
						"publish_time": int64(duration),
					})
				w.WritePoint(p)

				if i%500 == 0 {
					runenv.RecordMessage("published %d items (series: %s)", i, tst.Name)
				}
			}
		}
		// signal to subscribers they can start.
		_, err = writer.SignalEntry(ctx, handoff)
		if err != nil {
			return err
		}

		_, err = writer.SignalEntry(ctx, end)
		if err != nil {
			return err
		}

		// wait for everyone to be done; this is necessary because the sync
		// service applies TTL by electing the first 5 publishers on the Redis
		// Stream to own the keepalive. In this case, all 5 publishers will be
		// THE publisher. In an ordinary test case, each instance will write a
		// key and therefore the ownership will be distributed. That does not
		// happen here, as all key publishing is concentrated on the publisher.
		<-watcher.Barrier(ctx, end, int64(runenv.TestGroupInstanceCount))

	case "receive":
		defer func() {
			_, err := writer.SignalEntry(ctx, end)
			if err != nil {
				panic(err)
			}
		}()

		runenv.RecordMessage("i am a subscriber")

		// if we are receiving, wait for the publisher to be done.
		<-watcher.Barrier(ctx, handoff, int64(1))

		errcount := 0

		for _, tst := range tests {
			ch := make(chan *string, 1)
			err = watcher.Subscribe(ctx, tst.Subtree, ch)
			if err != nil {
				return err
			}
			for i := 1; i <= iterations; i++ {
				before := time.Now()
				b := <-ch
				duration := time.Since(before)
				if strings.Compare(*b, *tst.Data) != 0 {
					errcount += 1
				}
				if i%500 == 0 {
					runenv.RecordMessage("received %d items (series: %s)", i, tst.Name)
				}
				p := makePoint(runenv, "subtree subscribe time",
					map[string]interface{}{
						"size":           len(*tst.Data),
						"subscribe_time": int64(duration),
						"error_count":    errcount,
					})
				w.WritePoint(p)
			}
		}
	}

	return nil
}

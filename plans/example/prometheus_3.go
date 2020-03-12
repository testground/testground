package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	counter  prometheus.Counter
	counter2 prometheus.Counter
	stage1   prometheus.Histogram
	stage2   prometheus.Histogram
	stage3   prometheus.Histogram
)

func ExamplePrometheus3(runenv *runtime.RunEnv) error {
	rand.Seed(time.Now().UnixNano())

	counter = runtime.NewCounter(runenv, "anton_counter", "")
	counter2 = runtime.NewCounter(runenv, "anton_counter2", "")
	stage1 = runtime.NewHistogram(runenv, "anton_stage1_timer", "")
	stage2 = runtime.NewHistogram(runenv, "anton_stage2_timer", "")
	stage3 = runtime.NewHistogram(runenv, "anton_stage3_timer", "")

	// run test

	executeStage(100, 50, stage1)

	time.Sleep(30 * time.Second)

	executeStage(200, 20, stage2)

	time.Sleep(40 * time.Second)

	executeStage(500, 50, stage3)

	time.Sleep(20 * time.Second)

	executeCounterStep()

	return nil
}

func executeStage(initialLatency int, jitter int, s prometheus.Histogram) {
	var wg sync.WaitGroup
	wg.Add(100)

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			latency := initialLatency + rand.Intn(jitter)

			start := time.Now()

			time.Sleep(time.Duration(latency) * time.Millisecond)

			s.Observe(float64(time.Since(start)))
			counter.Inc()
		}()
	}

	wg.Wait()
}

func executeCounterStep() {
	for i := 0; i < 100; i++ {
		counter2.Inc()
		time.Sleep(2 * time.Second)
	}
}

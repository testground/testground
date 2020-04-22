package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/testground/testground/sdk/runtime"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	counter  runtime.Counter
	counter2 runtime.Counter
	stage1   runtime.Histogram
	stage2   runtime.Histogram
	stage3   runtime.Histogram
)

func ExamplePrometheus3(runenv *runtime.RunEnv) error {
	rand.Seed(time.Now().UnixNano())

	counter = runenv.M().NewCounter(runtime.CounterOpts{Name: "anton_counter"})
	counter2 = runenv.M().NewCounter(runtime.CounterOpts{Name: "anton_counter2"})
	stage1 = runenv.M().NewHistogram(runtime.HistogramOpts{Name: "anton_stage1_timer"})
	stage2 = runenv.M().NewHistogram(runtime.HistogramOpts{Name: "anton_stage2_timer"})
	stage3 = runenv.M().NewHistogram(runtime.HistogramOpts{Name: "anton_stage3_timer"})

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

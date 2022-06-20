package main

import (
	"math/rand"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// ExampleMetrics generates random data every 100 miliseconds and writes it to metrics for 30
// seconds. In order to see the output, plans should be run with the `--collect` option. The metrics
// are saved in a plain text file `metrics.out`
func ExampleMetrics(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		counter   = runenv.R().Counter("example.counter1")
		histogram = runenv.R().Histogram("example.histogram1", runenv.R().NewUniformSample(1028))
		gauge     = runenv.R().Gauge("example.gauge1")
	)

	rand.Seed(time.Now().UnixNano())

	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)

	go func() {
		time.Sleep(30 * time.Second)
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			data := int64(rand.Intn(15))
			runenv.RecordMessage("Doing work: %d", data)
			counter.Inc(data)
			histogram.Update(data)
			gauge.Update(float64(data))
		case <-done:
			return nil
		}

	}
}

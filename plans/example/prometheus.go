package main

import (
	"math/rand"
	"time"

	"github.com/testground/testground/sdk/runtime"
)

// the testground-ified version of this example:
// https://godoc.org/github.com/prometheus/client_golang/prometheus/push#example-Pusher-Add
func ExamplePrometheus(runenv *runtime.RunEnv) error {
	completionTime := runenv.M().NewGauge(runtime.GaugeOpts{
		Name: "db_backup_last_completion_time_seconds",
		Help: "The timestamp of the last completion of a DB backup, successful or not.",
	})
	successTime := runenv.M().NewGauge(runtime.GaugeOpts{
		Name: "db_backup_last_success_timestamp_seconds",
		Help: "The timestamp of the last successful completion of a DB backup.",
	})
	duration := runenv.M().NewGauge(runtime.GaugeOpts{
		Name: "db_backup_duration_seconds",
		Help: "The duration of the last DB backup in seconds.",
	})
	records := runenv.M().NewGauge(runtime.GaugeOpts{
		Name: "db_backup_records_processed",
		Help: "The number of records processed in the last DB backup.",
	})

	// Notice, you don't have to instantiate the pusher or push data yourself.
	// This is handled for you by runtime.Invoke()
	// Just create the collectors, and add to them as appropriate :)

	start := time.Now()

	// Do some work.
	// Following the example, we have backed up 42 records.
	n, err := func() (int, error) { return 42, nil }()

	records.Set(float64(n))
	duration.Set(time.Since(start).Seconds())
	completionTime.SetToCurrentTime()

	if err != nil {
		runenv.RecordFailure(err)
	} else {
		successTime.SetToCurrentTime()
	}

	return nil
}

// I want to demonstrate other kinds of prometheus metrics types,
// In this example, we have a long-ish running test which periodically updates metrics

// Here are some promql queries you can run in the prometheus dashboard, to get some ideas:
//
// Get the 50th percentile of samples over a 2-minute period:
// histogram_quantile(0.5, rate(example_histogram_bucket[2m])
//
// 90th percentile of histogram, by a grouping. In this case, the TestGroupId
// histogram_quantile(0.9, sum(rate(example_histogram_bucket[1m])) by (TestGroupId, le))
//
// Averages, sums, etc.
// avg(example_gauge)
// sum(example_gauge)
//
// if you only care about the top K performers:
// topk(5, example_counter2)
// Or the bottom k:
// bottomk(5, example_counter2)
// How much difference is there?
// stddev(example_gauge2)
// stdvar(example_gauge)
//
// For more examples, see https://prometheus.io/docs/prometheus/latest/querying/basics/
func ExamplePrometheus2(runenv *runtime.RunEnv) error {
	counter := runenv.M().NewCounter(runtime.CounterOpts{Name: "example_counter", Help: "I count how many times something happens"})
	counter2 := runenv.M().NewCounter(runtime.CounterOpts{Name: "example_counter2", Help: "I count how many times something happens"})
	histogram := runenv.M().NewHistogram(runtime.HistogramOpts{Name: "example_histogram", Help: "information in buckets"})
	histogram2 := runenv.M().NewHistogram(runtime.HistogramOpts{Name: "example_histogram2", Help: "histogram with non-default buckets", Buckets: []float64{1.0, 5.0, 6.0}})
	gauge := runenv.M().NewGauge(runtime.GaugeOpts{Name: "example_gauge", Help: "values, can go up and down"})
	gauge2 := runenv.M().NewGauge(runtime.GaugeOpts{Name: "example_gauge2", Help: "values, can go up and down"})
	rand.Seed(time.Now().UnixNano())

	// increment the counter once per second
	// Also record a random number into each of the metrics
	for i := 0; i <= 600; i++ {
		time.Sleep(time.Second)
		data := float64(rand.Intn(15))
		runenv.RecordMessage("Doing work: %f", data)
		counter.Inc()
		counter2.Add(data)
		// gauge also has Inc, Sub, etc.
		gauge.Set(data)
		gauge2.Add(data)
		// Histograms place data into buckets,
		// Observations are counted depending on which bucket the data falls within.
		histogram.Observe(data)
		histogram2.Observe(data)
	}
	return nil
}

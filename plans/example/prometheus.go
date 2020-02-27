package main

import (
	"time"

	"github.com/ipfs/testground/sdk/runtime"
)

// the testrgound-ified version of this example:
// https://godoc.org/github.com/prometheus/client_golang/prometheus/push#example-Pusher-Add
func ExamplePrometheus(runenv *runtime.RunEnv) error {
	completionTime := runtime.NewGauge(
		runenv,
		"db_backup_last_completion_time_seconds",
		"The timestamp of the last completion of a DB backup, successful or not.")
	successTime := runtime.NewGauge(
		runenv,
		"db_backup_last_success_timestamp_seconds",
		"The timestamp of the last successful completion of a DB backup.")
	duration := runtime.NewGauge(
		runenv,
		"db_backup_duration_seconds",
		"The duration of the last DB backup in seconds.")
	records := runtime.NewGauge(
		runenv,
		"db_backup_records_processed",
		"The number of records processed in the last DB backup.")

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

package runtime

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Invoke runs the passed test-case and reports the result.
func Invoke(tc func(*RunEnv) error) {
	runenv := CurrentRunEnv()
	defer runenv.MetricsPusher.Push()
	defer runenv.Close()

	startGauge := NewGauge(runenv, "start_time", "time of plan start")
	startGauge.SetToCurrentTime()
	runenv.RecordStart()

	endGauge := NewGauge(runenv, "end_time", "time of plan end")
	errfile, err := runenv.CreateRawAsset("run.err")
	if err != nil {
		runenv.RecordCrash(err)
		endGauge.SetToCurrentTime()
		return
	}

	rd, wr, err := os.Pipe()
	if err != nil {
		runenv.RecordCrash(err)
		endGauge.SetToCurrentTime()
		return
	}

	w := io.MultiWriter(errfile, os.Stderr)
	os.Stderr = wr

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)

		_, err := io.Copy(w, rd)
		if err != nil && !strings.Contains(err.Error(), "file already closed") {
			runenv.RecordCrash(fmt.Errorf("stderr copy failed: %w", err))
			endGauge.SetToCurrentTime()
			return
		}

		if err = errfile.Sync(); err != nil {
			runenv.RecordCrash(fmt.Errorf("stderr file tee sync failed failed: %w", err))
			endGauge.SetToCurrentTime()
			return
		}
	}()

	// Prepare the event.
	defer func() {
		if err := recover(); err != nil {
			// Handle panics.
			runenv.RecordCrash(err)
			endGauge.SetToCurrentTime()
		}
	}()

	err = tc(runenv)
	switch err {
	case nil:
		runenv.RecordSuccess()
		endGauge.SetToCurrentTime()
	default:
		runenv.RecordFailure(err)
		endGauge.SetToCurrentTime()
	}

	_ = rd.Close()
	<-doneCh
}

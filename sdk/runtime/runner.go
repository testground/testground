package runtime

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// These ports are the HTTP ports we'll attempt to bind to. If this instance
	// is running in a Docker container, binding to 6060 is safe. If it's a
	// local:exec run, these ports belong to the host, so starting more than one
	// instance will lead to a collision. Therefore we fallback to 0.
	HTTPPort         = 6060
	HTTPPortFallback = 0
)

// HTTPListenAddr will be set to the listener address _before_ the test case is
// invoked. If we were unable to start the listener, this value will be "".
var HTTPListenAddr string

// Invoke runs the passed test-case and reports the result.
func Invoke(tc func(*RunEnv) error) {
	var (
		runenv        = CurrentRunEnv()
		start         = time.Now()
		durationGauge = NewGauge(runenv, "plan_duration", "Run time (seconds)")
	)

	setupHTTPListener(runenv)

	// The prometheus pushgateway has a customized scrape interval, which is used to hint to the
	// prometheus operator at which interval the it should be scraped. This is currently set to 5s.
	// To provide an updated metric in every scrape, jobs will push to the pushgateway at the same
	// interval. When this "pushInterval" is changed, you may want to change the scrape interval
	// on the pushgateway
	pushStopCh := make(chan struct{})
	go func() {
		pushInterval := 5 * time.Second

		// Wait until the pushgateway is ready.
		runenv.RecordMessage("Waiting for pushgateway to become accessible.")
		var resbuf []byte
		for {
			select {
			case <-time.After(pushInterval):
				resp, err := http.Get("http://prometheus-pushgateway:9091/-/ready")
				if err != nil {
					continue
				}
				resp.Body.Read(resbuf)
				if string(resbuf) == "OK" {
					break
				}
			case <-pushStopCh:
				return
			}
		}

		runenv.RecordMessage("pushgateway is up. Pushing metrics every %d seconds.", pushInterval)

		for {
			select {
			case <-time.After(pushInterval):
				err := runenv.MetricsPusher.Add()
				if err != nil {
					runenv.RecordMessage("error during periodic metric push: %w", err)
				}
			case <-pushStopCh:
				return
			}
		}
	}()

	// Push metrics one last time, including the duration for the whole run.
	defer func() {
		defer close(pushStopCh)

		durationGauge.Set(time.Since(start).Seconds())
		err := runenv.MetricsPusher.Add()
		if err != nil {
			runenv.RecordMessage("error during end metric push: %w", err)
		}
	}()

	defer runenv.Close()

	runenv.RecordStart()

	errfile, err := runenv.CreateRawAsset("run.err")
	if err != nil {
		runenv.RecordCrash(err)
		return
	}

	rd, wr, err := os.Pipe()
	if err != nil {
		runenv.RecordCrash(err)
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
			return
		}

		if err = errfile.Sync(); err != nil {
			runenv.RecordCrash(fmt.Errorf("stderr file tee sync failed failed: %w", err))
		}
	}()

	// Prepare the event.
	defer func() {
		if err := recover(); err != nil {
			// Handle panics by recording them in the runenv output.
			runenv.RecordCrash(err)

			// Developers expect panics to be recorded in run.err too.
			fmt.Fprintln(os.Stderr, err)
			debug.PrintStack()
		}
	}()

	err = tc(runenv)
	switch err {
	case nil:
		runenv.RecordSuccess()
	default:
		runenv.RecordFailure(err)
	}

	_ = rd.Close()
	<-doneCh
}

func setupHTTPListener(runenv *RunEnv) {
	addr := fmt.Sprintf("0.0.0.0:%d", HTTPPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		addr = fmt.Sprintf("0.0.0.0:%d", HTTPPortFallback)
		if l, err = net.Listen("tcp", addr); err != nil {
			runenv.RecordMessage("error registering default http handler at: %s: %s", addr, err)
			return
		}
	}

	// DefaultServeMux already includes the pprof handler, add the
	// Prometheus handler.
	http.DefaultServeMux.Handle("/metrics", promhttp.Handler())

	HTTPListenAddr = l.Addr().String()

	runenv.RecordMessage("registering default http handler at: http://%s/ (pprof: http://%s/debug/pprof/)", HTTPListenAddr, HTTPListenAddr)

	go http.Serve(l, nil)
}

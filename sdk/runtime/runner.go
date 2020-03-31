package runtime

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	// These ports are the HTTP ports we'll attempt to bind to. If this instance
	// is running in a Docker container, binding to 6060 is safe. If it's a
	// local:exec run, these ports belong to the host, so starting more than one
	// instance will lead to a collision. Therefore we fallback to 0.
	HTTPPort         = 6060
	HTTPPortFallback = 0

	MetricsPushInterval = 5 * time.Second
)

// PushgatewayEndpoints are endpoints to test before activating pushgateway.
var PushgatewayEndpoints = []string{"prometheus-pushgateway:9091", "localhost:9091"}

// HTTPListenAddr will be set to the listener address _before_ the test case is
// invoked. If we were unable to start the listener, this value will be "".
var HTTPListenAddr string

// Invoke runs the passed test-case and reports the result.
func Invoke(tc func(*RunEnv) error) {
	runenv := CurrentRunEnv()

	defer runenv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	setupHTTPListener(runenv)
	metricsDoneCh, pusher := setupMetrics(ctx, runenv)

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGTERM)

	go func() {
		<-termCh
		gracefulShutdown(cancel, runenv, pusher)
	}()

	defer gracefulShutdown(cancel, runenv, pusher)

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

	ioDoneCh := make(chan struct{})
	go func() {
		defer close(ioDoneCh)

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
	<-ioDoneCh
	<-metricsDoneCh
}

// setupMetrics tracks the test duration, and sets up Prometheus metrics push.
func setupMetrics(ctx context.Context, runenv *RunEnv) (doneCh chan error, pusher *push.Pusher) {
	doneCh = make(chan error)
	// Wait until the pushgateway is ready.
	runenv.RecordMessage("waiting for pushgateway to become accessible")

	tick := time.NewTicker(MetricsPushInterval)
	defer tick.Stop()

	var endpoint string
Outer:
	for b := make([]byte, 2); ; {
		for _, endpoint = range PushgatewayEndpoints {
			resp, err := http.Get(fmt.Sprintf("http://%s/-/ready", endpoint))
			if err != nil {
				continue
			}
			_, _ = resp.Body.Read(b)
			if string(b) == "OK" {
				break Outer
			}
		}

		select {
		case <-tick.C:
			// loop over
		case <-ctx.Done():
			// pushgateway was never ready.
			return
		}
	}

	runenv.RecordMessage("pushgateway is up at %s; pushing metrics every %s.", endpoint, MetricsPushInterval)

	hostname, _ := os.Hostname()

	pusher = push.New(endpoint, "testground/plan").
		Gatherer(prometheus.DefaultGatherer).
		Grouping("plan", runenv.TestPlan).
		Grouping("case", runenv.TestCase).
		Grouping("run_id", runenv.TestRun).
		Grouping("group_id", runenv.TestGroupID).
		Grouping("container_name", hostname)
	testDuration := runenv.M().NewGauge(prometheus.GaugeOpts{
		Name: "tg_plan_duration",
		Help: "test plan run time (seconds)",
	})

	durationCh := make(chan struct{})
	// Keep reporting the test duration every second.
	go func() {
		defer close(durationCh)

		t := prometheus.NewTimer(prometheus.ObserverFunc(testDuration.Set))
		defer t.ObserveDuration() // record before exiting.

		tick := time.NewTicker(1 * time.Second)
		defer tick.Stop()

		for {
			select {
			case <-tick.C:
				t.ObserveDuration()
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer close(doneCh)

		push := func() {
			if err := pusher.Add(); err != nil {
				runenv.RecordMessage("error during periodic metric push: %s", err)
			}
		}

		// push now
		push()

		// Push every MetricsPushInterval.
		for {
			select {
			case <-tick.C:
				push()
			case <-ctx.Done():
				return
			}
		}
	}()

	return
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

	go func() {
		_ = http.Serve(l, nil)
	}()
}

func gracefulShutdown(cancel context.CancelFunc, runenv *RunEnv, pusher *push.Pusher) {
	runenv.Message("shudown initiated")
	_ = pusher.Add()
	cancel()
	time.Sleep(30)
	_ = pusher.Delete()
}

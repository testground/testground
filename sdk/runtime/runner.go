package runtime

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime/debug"
	"strings"

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

type TestCaseFn func(env *RunEnv) error

// InvokeMap takes a map of test case names and their functions, and calls the
// matched test case, or panics if the name is unrecognised.
func InvokeMap(cases map[string]TestCaseFn) {
	Invoke(func(runenv *RunEnv) error {
		name := runenv.TestCase
		if c, ok := cases[name]; ok {
			return c(runenv)
		} else {
			panic(fmt.Sprintf("unrecognized test case: %s", name))
		}
	})
}

// Invoke runs the passed test-case and reports the result.
func Invoke(tc TestCaseFn) {
	runenv := CurrentRunEnv()
	defer runenv.Close()

	setupHTTPListener(runenv)

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
	runenv.RecordMessage("io closed")
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

package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/pborman/uuid"
)

type Daemon struct {
	server *http.Server
	l      net.Listener
	doneCh chan struct{}
}

// New creates a new Daemon and attaches the following handlers:
//
// * GET /list: sends a `list` request to the daemon. list all test plans and test cases.
// * GET /describe: sends a `describe` request to the daemon. describes a test plan or test case.
// * POST /build: sends a `build` request to the daemon. builds a test plan.
// * POST /run: sends a `run` request to the daemon. (builds and) runs test case with name `<testplan>/<testcase>`.
// A type-safe client for this server can be found in the `pkg/client` package.
func New(listenAddr string) (srv *Daemon, err error) {
	srv = new(Daemon)

	engine, err := engine.NewDefaultEngine()
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter()

	// Set a unique request ID.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New()[:8])
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/list", srv.listHandler(engine)).Methods("GET")
	r.HandleFunc("/describe", srv.describeHandler(engine)).Methods("GET")
	r.HandleFunc("/build", srv.buildHandler(engine)).Methods("POST")
	r.HandleFunc("/run", srv.runHandler(engine)).Methods("POST")
	r.HandleFunc("/outputs", srv.outputsHandler(engine)).Methods("POST")
	r.HandleFunc("/terminate", srv.terminateHandler(engine)).Methods("POST")

	srv.doneCh = make(chan struct{})
	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 600 * time.Second,
		ReadTimeout:  600 * time.Second,
	}

	srv.l, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	return srv, nil
}

// Serve starts the server and blocks until the server is closed, either
// explicitly via Shutdown, or due to a fault condition. It propagates the
// non-nil err return value from http.Serve.
func (s *Daemon) Serve() error {
	select {
	case <-s.doneCh:
		return fmt.Errorf("tried to reuse a stopped server")
	default:
	}

	logging.S().Infow("daemon listening", "addr", s.Addr())
	return s.server.Serve(s.l)
}

func (s *Daemon) Addr() string {
	return s.l.Addr().String()
}

func (s *Daemon) Port() int {
	return s.l.Addr().(*net.TCPAddr).Port
}

func (s *Daemon) Shutdown(ctx context.Context) error {
	defer close(s.doneCh)
	return s.server.Shutdown(ctx)
}

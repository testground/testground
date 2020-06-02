package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/engine"
	"github.com/testground/testground/pkg/logging"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
)

type Service struct {
	server *http.Server
	l      net.Listener
	doneCh chan struct{}
}

// New creates a new Daemon and attaches the following handlers:
//
// * POST /build: sends a `build` request to the daemon. builds a test plan.
// A type-safe client for this server can be found in the `pkg/client` package.
func New(cfg *config.EnvConfig) (srv *Service, err error) {
	srv = new(Service)

	engine, err := engine.NewServiceEngine(cfg)
	if err != nil {
		return nil, err
	}
	err = engine.TaskStorage().Open()
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

	// TODO Don't use /build,  /task POST
	r.HandleFunc("/build", srv.submitHandler(engine)).Methods("POST")
	r.HandleFunc("/task/{taskid}", srv.taskinfoHandler(engine)).Methods("GET")

	srv.doneCh = make(chan struct{})

	srv.server = &http.Server{
		Handler:      r,
		WriteTimeout: 1200 * time.Second,
		ReadTimeout:  1200 * time.Second,
	}

	srv.l, err = net.Listen("tcp", cfg.Daemon.Listen)
	if err != nil {
		return nil, err
	}

	return srv, nil
}

// Serve starts the server and blocks until the server is closed, either
// explicitly via Shutdown, or due to a fault condition. It propagates the
// non-nil err return value from http.Serve.
func (d *Service) Serve() error {
	select {
	case <-d.doneCh:
		return fmt.Errorf("tried to reuse a stopped server")
	default:
	}

	logging.S().Infow("daemon listening", "addr", d.Addr())
	return d.server.Serve(d.l)
}

func (d *Service) Addr() string {
	return d.l.Addr().String()
}

func (d *Service) Port() int {
	return d.l.Addr().(*net.TCPAddr).Port
}

func (d *Service) Shutdown(ctx context.Context) error {
	defer close(d.doneCh)
	return d.server.Shutdown(ctx)
}

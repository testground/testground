package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/ipfs/testground/pkg/engine"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/pborman/uuid"
	"go.uber.org/zap"
)

// _engine is the default engine shared by all commands.
var (
	_engine     *engine.Engine
	_engineErr  error
	_engineOnce sync.Once
)

func GetEngine() (*engine.Engine, error) {
	_engineOnce.Do(func() {
		_engine, _engineErr = engine.NewDefaultEngine()
	})
	return _engine, _engineErr
}

type Server struct {
	*http.Server
}

// New creates a new Server and attaches the following handlers:
// * GET /list: sends a `list` request to the daemon. list all test plans and test cases.
// * GET /describe: sends a `describe` request to the daemon. describes a test plan or test case.
// * POST /build: sends a `build` request to the daemon. builds a test plan.
// * POST /run: sends a `run` request to the daemon. (builds and) runs test case with name `<testplan>/<testcase>`.
// A type-safe client for this server can be found in the `client` package.
func New(listenAddr string) *Server {
	srv := &Server{}

	r := mux.NewRouter()
	r.HandleFunc("/list", loggingHandler(srv.listHandler)).Methods("GET")
	r.HandleFunc("/describe", loggingHandler(srv.describeHandler)).Methods("GET")
	r.HandleFunc("/build", loggingHandler(srv.buildHandler)).Methods("POST")
	r.HandleFunc("/run", loggingHandler(srv.runHandler)).Methods("POST")
	http.Handle("/", r)

	srv.Server = &http.Server{
		Handler:      r,
		Addr:         listenAddr,
		WriteTimeout: 600 * time.Second,
		ReadTimeout:  600 * time.Second,
	}

	return srv
}

func loggingHandler(f func(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", uuid.New()[:8])

		f(w, r, log)
	}
}

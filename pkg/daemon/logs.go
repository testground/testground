package daemon

import (
	"github.com/testground/testground/pkg/api"
	"net/http"
)

func (d *Daemon) logsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: tasks handler
		w.WriteHeader(http.StatusNotImplemented)
	}
}

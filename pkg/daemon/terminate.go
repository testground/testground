package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) terminateHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "terminate")
		defer log.Debugw("request handled", "command", "terminate")

		tgw := rpc.NewOutputWriter(w, r)

		var req api.TerminateRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("terminate json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = engine.DoTerminate(r.Context(), req.Runner, tgw)
		if err != nil {
			tgw.WriteError("terminate error", "err", err.Error())
			return
		}

		tgw.WriteResult("Done")
	}
}

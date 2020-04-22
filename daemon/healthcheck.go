package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/testground/testground/api"
	"github.com/testground/testground/logging"
	"github.com/testground/testground/rpc"
)

func (d *Daemon) healthcheckHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "healthcheck")
		defer log.Debugw("request handled", "command", "healthcheck")

		tgw := rpc.NewOutputWriter(w, r)

		var req api.HealthcheckRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("healthcheck json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		out, err := engine.DoHealthcheck(r.Context(), req.Runner, req.Fix, tgw)
		if err != nil {
			tgw.WriteError("healthcheck error", "err", err.Error())
			return
		}

		tgw.WriteResult(out)
	}
}

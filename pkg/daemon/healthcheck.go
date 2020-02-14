package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"
)

func (srv *Daemon) healthcheckHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "healthcheck")
		defer log.Debugw("request handled", "command", "healthcheck")

		tgw := tgwriter.New(w, log)

		var req client.HealthcheckRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("healthcheck json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		out, err := engine.DoHealthcheck(r.Context(), req.Runner, req.Repair, tgw)
		if err != nil {
			tgw.WriteError("healthcheck error", "err", err.Error())
			return
		}

		tgw.WriteResult(out)
	}
}

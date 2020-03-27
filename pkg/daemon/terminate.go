package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"
)

func (srv *Daemon) terminateHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "terminate")
		defer log.Debugw("request handled", "command", "terminate")

		tgw := tgwriter.New(w, r)

		var req client.TerminateRequest
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

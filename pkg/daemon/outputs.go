package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) outputsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "collect outputs")
		defer log.Debugw("request handled", "command", "collect outputs")

		var req api.OutputsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Errorw("collect outputs json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tgw := rpc.NewOutputWriter(w, r)

		result := false
		defer func() {
			tgw.WriteResult(result)
		}()

		err = engine.DoCollectOutputs(r.Context(), req.Runner, req.RunID, tgw)
		if err != nil {
			log.Warnw("collect outputs error", "err", err.Error())
			return
		}

		result = true
	}
}

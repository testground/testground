package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/rpc"
)

func (srv *Daemon) outputsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "collect outputs")
		defer log.Debugw("request handled", "command", "collect outputs")

		var req client.OutputsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Errorw("collect outputs json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tgw := rpc.NewOutputWriter(w, r)

		err = engine.DoCollectOutputs(r.Context(), req.Runner, req.RunID, tgw)
		if err != nil {
			log.Errorw("collect outputs error", "err", err.Error())
			return
		}
	}
}

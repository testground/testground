package daemon

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/logging"
)

func (srv *Daemon) outputsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "collect outputs")
		defer log.Debugw("request handled", "command", "collect outputs")

		var req client.OutputsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rc, err := engine.DoCollectOutputs(r.Context(), req.Runner, req.Run)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(w, rc)
		if err != nil {
			log.Errorw("error while copying archive", "err", err)
		}
	}
}

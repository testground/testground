package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) runHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Infow("handle request", "command", "run")
		defer log.Infow("request handled", "command", "run")

		tgw := rpc.NewOutputWriter(w, r)

		var req api.RunRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("cannot json decode request body", "err", err)
			return
		}

		out, err := engine.DoRun(r.Context(), &req.Composition, tgw)
		if err != nil {
			tgw.WriteError(fmt.Sprintf("engine run error: %s", err))
			return
		}

		tgw.WriteResult(out)
	}
}

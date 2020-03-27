package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"
)

func (srv *Daemon) buildHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "build")
		defer log.Debugw("request handled", "command", "build")

		tgw := tgwriter.New(w, r)

		var req client.BuildRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("cannot json decode request body", "err", err)
			return
		}

		out, err := engine.DoBuild(r.Context(), &req.Composition, tgw)
		if err != nil {
			tgw.WriteError(fmt.Sprintf("engine build error: %s", err))
			return
		}

		tgw.WriteResult(out)
	}
}

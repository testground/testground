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
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

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

		var (
			ctype api.ComponentType
			ref   string
		)

		switch {
		case req.Builder != "" && req.Runner != "":
			tgw.WriteError("cannot terminate a runner and a builder at the same time")
			return
		case req.Builder != "":
			ctype = api.BuilderType
			ref = req.Builder
		case req.Runner != "":
			ctype = api.RunnerType
			ref = req.Runner
		}

		err = engine.DoTerminate(r.Context(), ctype, ref, tgw)
		if err != nil {
			tgw.WriteError("terminate error", "err", err.Error())
			return
		}

		tgw.WriteResult("Done")
	}
}

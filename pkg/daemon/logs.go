package daemon

import (
	"encoding/json"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
	"net/http"
)

func (d *Daemon) logsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tgw := rpc.NewOutputWriter(w, r)

		var req api.LogsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("logs json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tsk, err := engine.Logs(r.Context(), req.TaskID, req.Follow, req.CancelWithContext, tgw)
		if err != nil {
			tgw.WriteError("error while fetching logs", "err", err.Error())
			return
		}

		tgw.WriteResult(tsk)
	}
}

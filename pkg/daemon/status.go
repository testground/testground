package daemon

import (
	"encoding/json"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
	"net/http"
)

func (d *Daemon) statusHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tgw := rpc.NewOutputWriter(w, r)

		var req api.StatusRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("status json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tsk, err := engine.TaskStatus(req.TaskID)
		if err != nil {
			tgw.Warnw("could not fetch status", "task_id", req.TaskID, "err", err)
			return
		}

		tgw.WriteResult(tsk)
	}
}

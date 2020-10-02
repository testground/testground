package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
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

		tsk, err := engine.GetTask(req.TaskID)
		if err != nil {
			tgw.Warnw("could not fetch status", "task_id", req.TaskID, "err", err)
			return
		}

		tgw.WriteResult(tsk)
	}
}

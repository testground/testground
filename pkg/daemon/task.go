package daemon

import (
	"encoding/json"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
	"net/http"
)

func (d *Daemon) taskStatusHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tgw := rpc.NewOutputWriter(w, r)

		var req api.TaskStatusRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("task status json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tsk, err := engine.TaskStatus(req.ID, req.WaitForCompletion)
		if err != nil {
			tgw.Warnw("could not fetch task status", "err", err)
			return
		}
		tgw.WriteResult(api.TaskStatusResponse{
			Priority:   tsk.Priority,
			ID:         tsk.ID,
			Type:       string(tsk.Type),
			Input:      tsk.Input,
			Result:     tsk.Result,
			Created:    tsk.Created().String(),
			LastUpdate: tsk.State().Created.String(),
			LastState:  string(tsk.State().TaskState),
		})
	}
}

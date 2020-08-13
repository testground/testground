package daemon

import (
	"github.com/gorilla/mux"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
	"net/http"
)

func (d *Daemon) taskStatusHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["taskid"]

		tgw := rpc.NewOutputWriter(w, r)

		tsk, err := engine.TaskStatus(id)
		if err != nil {
			tgw.Warnw("could not find task in storage", "err", err)
			return
		}
		tgw.WriteResult(api.TaskStatusResponse{
			Priority:   tsk.Priority,
			ID:         tsk.ID,
			Type:       string(tsk.Type),
			Input:      tsk.Input,
			Result:     tsk.Result,
			Created:    tsk.Created().String(),
			LastUpdate: tsk.LastState().Created.String(),
			LastState:  string(tsk.LastState().TaskState),
		})
	}
}

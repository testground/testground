package daemon

import (
	"github.com/gorilla/mux"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
	"net/http"
)

func (d *Daemon) taskinfoHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["taskid"]

		tgw := rpc.NewOutputWriter(w, r)

		tsk, err := engine.TaskStatus(id)
		if err != nil {
			tgw.Warnw("could not find task in storage", "err", err)
			return
		}
		tgw.WriteResult(tsk)
	}
}

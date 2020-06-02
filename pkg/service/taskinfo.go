package service

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Service) taskinfoHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		ruid := vars["taskid"]

		tgw := rpc.NewOutputWriter(w, r)

		tsk, err := engine.TaskStorage().Get(ruid)
		if err != nil {
			tgw.Warnw("could not find task in storage", "err", err)
			return
		}
		buf, err := json.Marshal(tsk)
		if err != nil {
			tgw.Warnw("could not unmarshal task", "err", err)
			return
		}

		tgw.Infof(string(buf))
	}
}

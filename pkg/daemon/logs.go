package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
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

func (d *Daemon) getLogsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "get logs")
		defer log.Debugw("request handled", "command", "get logs")

		w.Header().Set("Content-Type", "text/plain")

		taskId := r.URL.Query().Get("task_id")
		if taskId == "" {
			fmt.Fprintf(w, "url param `task_id` is missing")
			return
		}

		req := api.LogsRequest{
			TaskID: taskId,
			Follow: false,
		}

		rr, ww := io.Pipe()

		tgw := rpc.NewFileOutputWriter(ww)

		go func() {
			_, err := client.ParseLogsRequest(w, rr)
			if err != nil {
				fmt.Fprintf(w, "error while parsing logs: %s", err.Error())
			}
		}()

		_, err := engine.Logs(r.Context(), req.TaskID, req.Follow, req.CancelWithContext, tgw)
		if err != nil {
			fmt.Fprintf(w, "error while fetching logs: %s", err.Error())
			return
		}
	}
}

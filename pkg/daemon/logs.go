package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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

		tsk, err := engine.Logs(r.Context(), req.TaskID, req.Follow, req.CancelWithContext, w)
		if err != nil {
			tgw.WriteError("error while getting task", "err", err)
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

		path := filepath.Join(engine.EnvConfig().Dirs().Daemon(), taskId+".out")

		file, err := os.Open(path)
		if err != nil {
			log.Errorw("cannot open logs file", "err", err)
			return
		}
		defer file.Close()

		_, err = client.ParseLogsRequest(w, file)

		if err != nil && err != io.EOF {
			fmt.Fprintf(w, "error while parsing logs: %s", err.Error())
		}
	}
}

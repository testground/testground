package daemon

import (
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
)

func (d *Daemon) getJournalHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "get journal")
		defer log.Debugw("request handled", "command", "get journal")

		w.Header().Set("Content-Type", "text/plain")

		taskId := r.URL.Query().Get("task_id")
		if taskId == "" {
			fmt.Fprintf(w, "url param `task_id` is missing")
			return
		}

		tsk, err := engine.Status(taskId)
		if err != nil {
			fmt.Fprintf(w, "cannot fetch tsk")
			return
		}

		result := parseResult(tsk.Result)
		_, _ = w.Write([]byte(result.Journal))
	}
}

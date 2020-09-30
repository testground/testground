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

		result := decodeResultK8s(tsk.Result)
		if len(result.Journal.Events) > 0 {
			_, _ = w.Write([]byte("Kubernetes Events\n"))
			_, _ = w.Write([]byte("=================\n"))
		}
		for _, v := range result.Journal.Events {
			_, _ = w.Write([]byte(v))
		}

		if len(result.Journal.PodsStatuses) > 0 {
			_, _ = w.Write([]byte("Pods Statuses\n"))
			_, _ = w.Write([]byte("=================\n"))
		}
		for k := range result.Journal.PodsStatuses {
			_, _ = w.Write([]byte(k))
		}
	}
}

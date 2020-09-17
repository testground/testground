package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/task"
)

func (d *Daemon) tasksHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tgw := rpc.NewOutputWriter(w, r)

		var req api.TasksRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("tasks json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tasks, err := engine.Tasks(req)
		if err != nil {
			tgw.WriteError("tasks json decode", "err", err.Error())
			return
		}

		tgw.WriteResult(tasks)
	}
}

func (d *Daemon) listTasksHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "list tasks")
		defer log.Debugw("request handled", "command", "list tasks")

		w.Header().Set("Content-Type", "text/html")

		req := api.TasksRequest{
			Types:  []task.Type{task.TypeBuild, task.TypeRun},
			States: []task.State{task.StateScheduled, task.StateProcessing, task.StateComplete},
		}

		tasks, err := engine.Tasks(req)
		if err != nil {
			fmt.Fprintf(w, "tasks json decode error", err.Error())
			return
		}

		tf := "Mon Jan _2 15:04:05"

		fmt.Fprintf(w, "<table><th>task id</th><th>type</th><th>name</th><th>state</th><th>created</th><th>updated</td><th>outputs tgz</th><th>stdout/stderr</th><th>took</th>")
		for _, t := range tasks {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>stdout/stderr</a></td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.State().State, t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.Took())
		}
		fmt.Fprintf(w, "</table>")
	}
}

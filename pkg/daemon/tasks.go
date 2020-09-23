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
			fmt.Fprintf(w, "tasks json decode error: %s", err.Error())
			return
		}

		tf := "Mon Jan _2 15:04:05"

		fmt.Fprintf(w, "<table><th>task id</th><th>type</th><th>name</th><th>state</th><th>created</th><th>updated</td><th>outputs tgz</th><th>task logs</th><th>task journal</th><th>took</th><th>status</th>")
		for _, t := range tasks {
			if t.State().State == task.StateComplete {
				if t.Status { // green
					fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a></td><td><a href=/journal?task_id=%s>journal</a></td><td>%s</td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.State().State, t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, t.Took(), "&#9989;")
				} else {
					fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a><td><a href=/journal?task_id=%s>journal</a></td></td><td>%s</td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.State().State, t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, t.Took(), "&#10060;")
				}
			}

			if t.State().State == task.StateProcessing {
				fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a></td><td><a href=/journal?task_id=%s>journal</a></td><td></td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.State().State, t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, "&#128338;")
			}

			if t.State().State == task.StateScheduled {
				fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td></td><td></td><td></td><td></td><td></td></tr>", t.ID, t.Type, t.Name(), t.State().State, t.Created().Format(tf), t.State().Created.Format(tf))
			}
		}
		fmt.Fprintf(w, "</table>")
	}
}

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

		_, _ = w.Write([]byte(tsk.Journal))
	}
}

func (d *Daemon) redirect() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tasks", 301)
	}
}

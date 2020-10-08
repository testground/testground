package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/runner"
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

		cr, _ := engine.RunnerByName("cluster:k8s")
		rr := cr.(*runner.ClusterK8sRunner)
		allocatableCPUs, allocatableMemory, _ := rr.GetClusterCapacity()

		_, _ = w.Write([]byte("<strong>cluster resources</strong><br/>"))
		_, _ = w.Write([]byte(fmt.Sprintf("capacity cpus: %d<br/>", allocatableCPUs)))
		_, _ = w.Write([]byte(fmt.Sprintf("capacity memory: %s<br/>", humanize.Bytes(uint64(allocatableMemory)))))

		tf := "Mon Jan _2 15:04:05"

		fmt.Fprintf(w, "<table><th>task id</th><th>type</th><th>name</th><th>created</th><th>updated</td><th>outputs tgz</th><th>task logs</th><th>task journal</th><th>took</th><th>status</th><th>outcomes</th><th>error</th><th>actions</th><th>created by</th>")
		for _, t := range tasks {
			result := decodeResultK8s(t.Result)
			if t.State().State == task.StateCanceled {
				fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a><td><a href=/journal?task_id=%s>journal</a></td></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, t.Took(), "&#9898;", result, t.Error, t.ParseCreatedBy())
			}

			if t.State().State == task.StateComplete {
				if result.Status { // green
					fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a></td><td><a href=/journal?task_id=%s>journal</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, t.Took(), "&#9989;", result, t.Error, t.ParseCreatedBy())
				} else { // red
					fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a><td><a href=/journal?task_id=%s>journal</a></td></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td><td>%s</td></tr>", t.ID, t.Type, t.Name(), t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, t.Took(), "&#10060;", result, t.Error, t.ParseCreatedBy())
				}
			}

			if t.State().State == task.StateProcessing {
				fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td><a href=/outputs?run_id=%s>download</a></td><td><a href=/logs?task_id=%s>logs</a></td><td><a href=/journal?task_id=%s>journal</a></td><td></td><td>%s</td><td></td><td></td><td><a href=/kill?task_id=%s>kill</a> | <a onclick="return confirm('Are you sure?');" href=/delete?task_id=%s>delete</a></td><td>%s</td></tr>`, t.ID, t.Type, t.Name(), t.Created().Format(tf), t.State().Created.Format(tf), t.ID, t.ID, t.ID, "&#9203;", t.ID, t.ID, t.ParseCreatedBy())
			}

			if t.State().State == task.StateScheduled {
				fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%v</td><td>%s</td><td></td><td></td><td></td><td></td><td>%s</td><td></td><td></td><td><a href=/kill?task_id=%s>kill</a> | <a onclick="return confirm('Are you sure?');" href=/delete?task_id=%s>delete</a></td><td>%s</td></tr>`, t.ID, t.Type, t.Name(), t.Created().Format(tf), t.State().Created.Format(tf), "&#128338;", t.ID, t.ID, t.ParseCreatedBy())
			}
		}
		fmt.Fprintf(w, "</table>")
	}
}

func (d *Daemon) redirect() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tasks", 301)
	}
}

func decodeResultK8s(result interface{}) *runner.ResultK8s {
	r := &runner.ResultK8s{}
	err := mapstructure.Decode(result, r)
	if err != nil {
		logging.S().Errorw("error while decoding result", "err", err)
	}
	return r
}

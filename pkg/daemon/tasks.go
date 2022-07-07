package daemon

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/data"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
	"github.com/testground/testground/tmpl"
)

const (
	EmojiSuccess    string = "&#9989;"
	EmojiCanceled   string = "&#9898;"
	EmojiFailure    string = "&#10060;"
	EmojiInProgress string = "&#9203;"
	EmojiScheduled  string = "&#128338;"
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

		before := time.Now().Add(-7 * 24 * time.Hour)
		req := api.TasksRequest{
			Types:  []task.Type{task.TypeBuild, task.TypeRun},
			States: []task.State{task.StateScheduled, task.StateProcessing, task.StateComplete},
			Before: &before,
		}

		tasks, err := engine.Tasks(req)
		if err != nil {
			fmt.Fprintf(w, "tasks json decode error: %s", err.Error())
			return
		}

		cr, _ := engine.RunnerByName("cluster:k8s")
		rr := cr.(*runner.ClusterK8sRunner)

		var allocatableCPUs, allocatableMemory int64
		if rr.Enabled() {
			allocatableCPUs, allocatableMemory, _ = rr.GetClusterCapacity()
		}

		tdata := struct {
			Tasks          []interface{}
			ClusterEnabled bool
			CPUs           string
			Memory         string
		}{
			nil,
			rr.Enabled(),
			fmt.Sprintf("%d", allocatableCPUs),
			humanize.Bytes(uint64(allocatableMemory)),
		}

		tf := "Mon Jan _2 15:04:05"

		for _, t := range tasks {
			outcome, err := data.DecodeTaskOutcome(&t)
			if err != nil {
				panic(fmt.Sprintf("cannot decode task outcome %s: %s", t.ID, err.Error()))
			}

			result := data.DecodeRunnerResult(t.Result)

			currentTask := struct {
				ID        string
				Name      string
				Created   string
				Updated   string
				Took      string
				Outcomes  string
				Status    string
				Error     string
				Actions   string
				CreatedBy string
			}{
				t.ID,
				t.Name(),
				t.Created().Format(tf),
				t.State().Created.Format(tf),
				t.Took().String(),
				result.StringOutcomes(),
				"",
				t.Error,
				"",
				t.RenderCreatedBy(),
			}

			switch t.State().State {
			case task.StateComplete:
				switch outcome {
				case task.OutcomeSuccess:
					currentTask.Status = EmojiSuccess
				case task.OutcomeFailure:
					currentTask.Status = EmojiFailure
				default:
					currentTask.Status = EmojiFailure
				}
			case task.StateCanceled:
				currentTask.Status = EmojiCanceled
			case task.StateProcessing:
				currentTask.Status = EmojiInProgress
				currentTask.Actions = fmt.Sprintf(`<a href=/kill?task_id=%s>kill</a><br/><a onclick="return confirm('Are you sure?');" href=/delete?task_id=%s>delete</a>`, t.ID, t.ID)
				currentTask.Took = ""
			case task.StateScheduled:
				currentTask.Status = EmojiScheduled
				currentTask.Took = ""
			}

			tdata.Tasks = append(tdata.Tasks, currentTask)
		}

		t := template.New("tasks.html").Funcs(template.FuncMap{"unescape": unescape})
		content, err := tmpl.HtmlTemplates.ReadFile("tasks.html")
		if err != nil {
			panic(fmt.Sprintf("cannot find template file: %s", err))
		}
		t, err = t.Parse(string(content))
		if err != nil {
			panic(fmt.Sprintf("cannot ParseFiles with tmpl/tasks: %s", err))
		}

		err = t.Execute(w, tdata)
		if err != nil {
			panic(fmt.Sprintf("cannot execute template: %s", err))
		}
	}
}

func (d *Daemon) redirect() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tasks", 301)
	}
}

func unescape(s string) template.HTML {
	return template.HTML(s)
}

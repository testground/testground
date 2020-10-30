package daemon

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
)

type Item struct {
	Id      string
	Title   string
	Series  string
	RootURL string
	Unit    string
}

func (d *Daemon) dashboardHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "dashboard task")
		defer log.Debugw("request handled", "command", "dashboard task")

		taskId := r.URL.Query().Get("task_id")
		if taskId == "" {
			fmt.Fprintf(w, "url param `task_id` is missing")
			return
		}

		tsk, err := engine.GetTask(taskId)
		if err != nil {
			fmt.Fprintf(w, "Cannot get task")
			return
		}

		name := clean(tsk.Plan) + "-" + tsk.Case

		measurements, err := d.mv.GetMeasurements(name)
		if err != nil {
			fmt.Fprintf(w, "Cannot get measurements")
			return
		}

		if measurements == nil {
			fmt.Fprintf(w, "No measurements for this test plan.")
			return
		}

		t := template.New("measurements.html")
		t, err = t.ParseFiles("tmpl/measurements.html")
		if err != nil {
			panic(err)
		}

		data := struct {
			Plan  string
			Items []Item
		}{
			tsk.Plan + ":" + tsk.Case,
			nil,
		}

		for i, m := range measurements {
			split := strings.Split(m, ".")
			d := Item{
				Title:   split[2],
				Series:  m,
				Unit:    split[len(split)-2],
				Id:      fmt.Sprintf("measurement_%d", i),
				RootURL: engine.EnvConfig().Daemon.RootURL,
			}
			data.Items = append(data.Items, d)
		}

		err = t.Execute(w, data)
		if err != nil {
			panic(err)
		}
	}
}

func clean(name string) string {
	forbiddenChar := "/"

	name = strings.Replace(name, forbiddenChar, "-", -1)

	return name
}

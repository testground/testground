package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/task"
)

type aggregatePoint struct {
	datetime int64
	total    int64
	n        int64
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func (d *Daemon) dashboardHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "dashboard task")
		defer log.Debugw("request handled", "command", "dashboard task")

		w.Header().Set("Content-Type", "text/plain")
		enableCors(&w)

		testplan := r.URL.Query().Get("testplan")
		if testplan == "" {
			fmt.Fprintf(w, "query param `testplan` is missing")
			return
		}

		testcase := r.URL.Query().Get("testcase")
		if testcase == "" {
			fmt.Fprintf(w, "query param `testcase` is missing")
			return
		}

		series := r.URL.Query().Get("series")
		if series == "" {
			fmt.Fprintf(w, "query param `series` is missing")
			return
		}

		req := api.TasksRequest{
			Types:    []task.Type{task.TypeRun},
			States:   []task.State{task.StateComplete},
			TestPlan: testplan,
			TestCase: testcase,
		}
		tasks, err := engine.Tasks(req)
		if err != nil {
			fmt.Fprintf(w, "cannot get tasks based on filters")
			return
		}

		log.Debugw("found tasks", "len", len(tasks), "testplan", testplan, "testcase", testcase)

		// final data rendered on chart
		data := make(map[string][]aggregatePoint)

		for _, t := range tasks {
			if t.IsCanceled() {
				log.Debugw("canceled task", "id", t.ID)
				continue
			}
			log.Debugw("processing task", "id", t.ID)
			file := fmt.Sprintf("/efs/%s/requestors/0/results.out", t.ID)
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(w, "cannot open file: %s ; %s", file, err)
				return
			}

			taskPoints := make(map[string]aggregatePoint)

			var msg runtime.Metric
			for dec := json.NewDecoder(f); ; {
				err := dec.Decode(&msg)
				if err != nil && err != io.EOF {
					fmt.Fprintf(w, "err from dec.Decode: %s", err)
					break
				}
				if err == io.EOF {
					break
				}

				pointseries, value := processDurationPoint(msg)
				log.Debugw("point", "datetime", t.Created(), "pointseries", pointseries, "value", value)
				if p, ok := taskPoints[pointseries]; ok {
					taskPoints[pointseries] = aggregatePoint{t.Created().UnixNano(), p.total + value, p.n + 1}
				} else {
					taskPoints[pointseries] = aggregatePoint{t.Created().UnixNano(), value, 1}
				}
			}

			// append the aggregate point
			for k, v := range taskPoints {
				data[k] = append(data[k], v)
			}

		}
		log.Debugw("outputing", "series", series)

		// table header
		s := fmt.Sprintf("Date,%s\n", strings.Replace(series, ",", " ", -1))
		_, _ = w.Write([]byte(s))

		log.Debugw("table data", "series", series)
		// table data
		for _, p := range data[series] {
			//s := fmt.Sprintf("%s,%d\n", p.datetime.Format("2006-01-02T15:04:05"), int64(p.value))
			s := fmt.Sprintf("%d,%d\n", p.datetime/10e5, p.total/p.n)
			_, _ = w.Write([]byte(s))
		}
		log.Debugw("done processing tasks", "testplan", testplan, "testcase", testcase)
	}
}

func processDurationPoint(msg runtime.Metric) (series string, value int64) {
	series = msg.Name[len("duration,"):]
	v, _ := msg.Measures["value"]
	value = int64(v.(float64))
	return
}

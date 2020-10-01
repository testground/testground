package daemon

import (
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
)

func (d *Daemon) killTaskHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "kill task")
		defer log.Debugw("request handled", "command", "kill task")

		w.Header().Set("Content-Type", "text/html")

		taskId := r.URL.Query().Get("task_id")
		if taskId == "" {
			fmt.Fprintf(w, "url param `task_id` is missing")
			return
		}

		err := engine.Kill(taskId)
		if err != nil {
			fmt.Fprintf(w, "cannot kill tsk")
			return
		}

		redirect := `
      <script>
         setTimeout(function(){
            window.location.href = 'https://ci.testground.ipfs.team/tasks';
         }, 1000);
      </script>
			`

		fmt.Fprintf(w, redirect)
	}
}

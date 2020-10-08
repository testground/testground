package daemon

import (
	"fmt"
	"net/http"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
)

func (d *Daemon) deleteHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "delete task")
		defer log.Debugw("request handled", "command", "delete task")

		w.Header().Set("Content-Type", "text/html")

		taskId := r.URL.Query().Get("task_id")
		if taskId == "" {
			fmt.Fprintf(w, "url param `task_id` is missing")
			return
		}

		err := engine.DeleteTask(taskId)
		if err != nil {
			fmt.Fprintf(w, "cannot delete tsk")
			return
		}

		redirect := `
      <script>
         setTimeout(function(){
            window.location.href = 'https://ci.testground.ipfs.team/tasks';
         }, 1000);
      </script>
			`

		fmt.Fprint(w, redirect)
	}
}

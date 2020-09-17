package daemon

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) runHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ruid := r.Header.Get("X-Request-ID")
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Infow("handle request", "command", "run")
		defer log.Infow("request handled", "command", "run")

		tgw := rpc.NewOutputWriter(w, r)

		// Create a packing directory under the workdir.
		dir := filepath.Join(engine.EnvConfig().Dirs().Work(), "requests", ruid)
		if err := os.MkdirAll(dir, 0755); err != nil {
			tgw.WriteError("failed to create temp directory to unpack request", "err", err)
			return
		}

		var request *api.RunRequest
		sources, err := consumeRunBuildRequest(r, &request, dir)
		if err != nil {
			tgw.WriteError("failed to consume request", "err", err)
			return
		}

		if len(request.BuildGroups) > 0 && sources == nil {
			tgw.WriteError("failed to consume request", "err", errors.New("plan dir required for build"))
			return
		}

		id, err := engine.QueueRun(request, sources)
		if err != nil {
			tgw.WriteError(fmt.Sprintf("engine run error: %s", err))
			return
		}

		tgw.WriteResult(id)
	}
}

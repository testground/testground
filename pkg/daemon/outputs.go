package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/client"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) outputsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "collect outputs")
		defer log.Debugw("request handled", "command", "collect outputs")

		var req api.OutputsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Errorw("collect outputs json decode", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tgw := rpc.NewOutputWriter(w, r)

		result := false
		defer func() {
			tgw.WriteResult(result)
		}()

		err = engine.DoCollectOutputs(r.Context(), req.RunID, tgw)
		if err != nil {
			log.Warnw("collect outputs error", "err", err.Error())
			return
		}

		result = true
	}
}

func (d *Daemon) getOutputsHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("req_id", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "get outputs")
		defer log.Debugw("request handled", "command", "get outputs")

		runId := r.URL.Query().Get("run_id")
		if runId == "" {
			fmt.Fprintf(w, "url param `run_id` is missing")
			return
		}

		w.Header().Set("Content-Type", "application/tar+gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tgz\"", runId))

		req := api.OutputsRequest{
			RunID: runId,
		}

		rr, ww := io.Pipe()

		tgw := rpc.NewFileOutputWriter(ww)

		go func() {
			_, err := client.ParseCollectResponse(rr, w, os.Stdout)
			if err != nil {
				fmt.Fprintf(w, "error while parsing collect response: %s", err.Error())
			}
		}()

		err := engine.DoCollectOutputs(r.Context(), req.RunID, tgw)
		if err != nil {
			log.Warnw("collect outputs error", "err", err.Error())
			return
		}
	}
}

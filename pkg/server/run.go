package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/pkg/ioutils"
	aapi "github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"go.uber.org/zap"
)

func (srv *Server) runHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "run")
	defer log.Debugw("request handled", "command", "run")

	w.Header().Set("Content-Type", "application/json")

	var req client.RunRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Errorw("cannot json decode request body", "err", err)
		return
	}

	engine, err := GetEngine()
	if err != nil {
		log.Errorw("get engine error", "err", err)
		return
	}

	runIn := &aapi.RunInput{
		Instances:    req.Instances,
		ArtifactPath: req.ArtifactPath,
		RunnerConfig: req.RunnerConfig, // cfgOverride,
		Parameters:   req.Parameters,
	}

	result, err := engine.DoRun(req.Plan, req.Case, req.Runner, runIn, ioutils.NewWriteFlusher(w))
	if err != nil {
		log.Errorw("engine run error", "err", err)
		return
	}

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		log.Errorw("encode error", "err", err, "result", result)
		return
	}
}

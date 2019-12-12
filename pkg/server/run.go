package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/pkg/ioutils"
	aapi "github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/tgwriter"
	"go.uber.org/zap"
)

func (srv *Server) runHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "run")
	defer log.Debugw("request handled", "command", "run")

	tgw := tgwriter.New(w, log)

	var req client.RunRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		tgw.WriteError("cannot json decode request body", "err", err)
		return
	}

	engine, err := GetEngine()
	if err != nil {
		tgw.WriteError("get engine error", "err", err)
		return
	}

	runIn := &aapi.RunInput{
		Instances:    req.Instances,
		ArtifactPath: req.ArtifactPath,
		RunnerConfig: req.RunnerConfig, // cfgOverride,
		Parameters:   req.Parameters,
		BuilderID:    req.BuilderID,
	}

	result, err := engine.DoRun(req.Plan, req.Case, req.Runner, runIn, ioutils.NewWriteFlusher(tgw))
	if err != nil {
		tgw.WriteError("engine run error", "err", err)
		return
	}

	tgw.WriteResult(result)
}

package server

import (
	"encoding/json"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/tgwriter"
	"go.uber.org/zap"
)

func (srv *Server) buildHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "build")
	defer log.Debugw("request handled", "command", "build")

	w.Header().Set("Content-Type", "application/json")

	var req client.BuildRequest
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

	in := &api.BuildInput{
		Dependencies: req.Dependencies,
		BuildConfig:  req.BuildConfig,
	}

	tgw := tgwriter.New(w)
	out, err := engine.DoBuild(req.Plan, req.Builder, in, tgw)
	if err != nil {
		log.Errorw("engine build error", "err", err)
		return
	}

	err = tgw.WriteResult(out)
	if err != nil {
		log.Errorw("engine write result error", "err", err)
		return
	}
}

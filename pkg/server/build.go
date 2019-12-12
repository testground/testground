package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/daemon/client"
	"github.com/ipfs/testground/pkg/tgwriter"
	"go.uber.org/zap"
)

func (srv *Server) buildHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "build")
	defer log.Debugw("request handled", "command", "build")

	tgw := tgwriter.New(w, log)

	var req client.BuildRequest
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

	in := &api.BuildInput{
		Dependencies: req.Dependencies,
		BuildConfig:  req.BuildConfig,
	}

	out, err := engine.DoBuild(req.Plan, req.Builder, in, tgw)
	if err != nil {
		tgw.WriteError(fmt.Sprintf("engine build error: %s", err))
		return
	}

	tgw.WriteResult(out)
}

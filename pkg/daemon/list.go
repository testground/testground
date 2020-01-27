package daemon

import (
	"net/http"

	"github.com/ipfs/testground/pkg/tgwriter"
	"go.uber.org/zap"
)

func (srv *Server) listHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "list")
	defer log.Debugw("request handled", "command", "list")

	tgw := tgwriter.New(w, log)

	engine, err := GetEngine()
	if err != nil {
		log.Errorw("get engine error", "err", err)
		return
	}

	plans := engine.TestCensus().ListPlans()
	for _, tp := range plans {
		for _, tc := range tp.TestCases {
			_, err := tgw.Write([]byte(tp.Name + "/" + tc.Name + "\n"))
			if err != nil {
				log.Errorf("could not write response back", "err", err)
			}
			w.(http.Flusher).Flush()
		}
	}

	tgw.WriteResult("Done")
}

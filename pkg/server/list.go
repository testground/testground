package server

import (
	"net/http"

	"go.uber.org/zap"
)

func (srv *Server) listHandler(w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	log.Debugw("handle request", "command", "list")
	defer log.Debugw("request handled", "command", "list")

	engine, err := GetEngine()
	if err != nil {
		log.Errorw("get engine error", "err", err)
		return
	}

	plans := engine.TestCensus().ListPlans()
	for _, tp := range plans {
		for _, tc := range tp.TestCases {
			_, err := w.Write([]byte(tp.Name + "/" + tc.Name + "\n"))
			if err != nil {
				log.Errorw("could not write response", "err", err)
			}

			w.(http.Flusher).Flush()
		}
	}
}

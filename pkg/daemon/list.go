package daemon

import (
	"net/http"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"
)

func (srv *Daemon) listHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "list")
		defer log.Debugw("request handled", "command", "list")

		tgw := tgwriter.New(w, log)

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
}

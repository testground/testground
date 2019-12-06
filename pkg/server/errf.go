package server

import (
	"github.com/ipfs/testground/pkg/tgwriter"
	"go.uber.org/zap"
)

//errf logs an error and forwards it back to the testground client
func errf(tgw *tgwriter.TgWriter, log *zap.SugaredLogger, err error, keysAndValues ...interface{}) {
	log.Warnw(err.Error(), keysAndValues...)
	err = tgw.WriteError(err.Error())
	if err != nil {
		log.Errorw("could not write error response", "err", err)
	}
}

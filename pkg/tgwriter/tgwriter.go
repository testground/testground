package tgwriter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/docker/docker/pkg/ioutils"

	"go.uber.org/zap"
)

func New(w http.ResponseWriter, log *zap.SugaredLogger) *TgWriter {
	w.Header().Set("Content-Type", "application/json")

	return &TgWriter{
		output: ioutils.NewWriteFlusher(w),
		log:    log,
	}
}

type TgWriter struct {
	sync.Mutex

	io.Writer
	output io.Writer
	log    *zap.SugaredLogger
}

// Msg defines a protocol message struct sent from the Testground daemon to the Testground client.
// For a given request, clients should expect between 1 and `n` `progress` messages, and
// exactly 1 `result` message.
type Msg struct {
	Type    string      `json:"type"` // progress or result or error
	Payload interface{} `json:"payload,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message"`
}

func (tgw *TgWriter) Write(p []byte) (n int, err error) {
	if p == nil {
		return 0, nil
	}

	pld := Msg{
		Type:    "progress",
		Payload: p,
	}

	json, err := json.Marshal(pld)
	if err != nil {
		return 0, err
	}

	tgw.Lock()
	defer tgw.Unlock()

	return tgw.output.Write(json)
}

func (tgw *TgWriter) WriteResult(res interface{}) {
	pld := Msg{
		Type:    "result",
		Payload: res,
	}

	json, err := json.Marshal(pld)
	if err != nil {
		tgw.log.Errorw("could not write result", "err", err)
		return
	}

	tgw.Lock()
	defer tgw.Unlock()

	_, err = tgw.output.Write(json)
	if err != nil {
		tgw.log.Errorw("could not write result", "err", err)
	}
}

func (tgw *TgWriter) WriteError(message string, keysAndValues ...interface{}) {
	tgw.log.Warnw(message, keysAndValues...)

	if len(keysAndValues) > 0 {
		b := &strings.Builder{}
		for i := 0; i < len(keysAndValues); i = i + 2 {
			fmt.Fprintf(b, "%s: %s;", keysAndValues[i], keysAndValues[i+1])
		}
		kvs := b.String()
		message = message + "; " + kvs[:len(kvs)-1]
	}

	pld := Msg{
		Type: "error",
		Error: &Error{
			Message: message,
		},
	}

	json, err := json.Marshal(pld)
	if err != nil {
		tgw.log.Errorw("could not write error response", "err", err)
		return
	}

	tgw.Lock()
	defer tgw.Unlock()

	_, err = tgw.output.Write(json)
	if err != nil {
		tgw.log.Errorw("could not write error response", "err", err)
	}
}

func (tgw *TgWriter) Flush() {
	if f, ok := tgw.output.(http.Flusher); ok {
		f.Flush()
	}
}

package tgwriter

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/ipfs/testground/pkg/logging"

	"github.com/docker/docker/pkg/ioutils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Msg defines a protocol message struct sent from the Testground daemon to the
// Testground client. For a given request, clients should expect between 0 to
// `n` `progress` messages, and exactly 1 `result` message.
type Msg struct {
	Type    string      `json:"type"` // progress or result or error
	Payload interface{} `json:"payload,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message"`
}

func New(w http.ResponseWriter, r *http.Request) *TgWriter {
	w.Header().Set("Content-Type", "application/json")

	httpWriter := ioutils.NewWriteFlusher(w)

	// progressWriter will emit log output as progress messages.
	progressWriter := &progressWriter{out: httpWriter}
	writeSyncer := zapcore.Lock(zapcore.AddSync(progressWriter))

	// this logger has two sinks: stdout and the writeSyncer, wired to the HTTP
	// response.
	logger := logging.NewLogger(writeSyncer).With(zap.String("req_id", r.Header.Get("X-Request-ID")))

	tgw := &TgWriter{
		SugaredLogger:  logger.Sugar(),
		out:            httpWriter,
		progressWriter: progressWriter,
	}

	// we need to wire this back for the lock.
	progressWriter.tgw = tgw
	return tgw
}

func Discard() *TgWriter {
	pw := &progressWriter{out: ioutil.Discard}
	tgw := &TgWriter{
		SugaredLogger:  zap.NewNop().Sugar(),
		out:            ioutil.Discard,
		progressWriter: pw,
	}
	tgw.progressWriter = pw
	return tgw
}

type progressWriter struct {
	tgw *TgWriter
	out io.Writer
}

var _ io.Writer = (*progressWriter)(nil)

// Write on the logWriter wraps the incoming write into a progress message.
func (w *progressWriter) Write(p []byte) (n int, err error) {
	if p == nil {
		return 0, nil
	}

	msg := Msg{Type: "progress", Payload: p}
	json, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}

	w.tgw.Lock()
	defer w.tgw.Unlock()

	return w.out.Write(json)
}

type TgWriter struct {
	sync.Mutex
	*zap.SugaredLogger
	*progressWriter

	out io.Writer
}

func (tgw *TgWriter) WriteResult(res interface{}) {
	msg := Msg{Type: "result", Payload: res}
	json, err := json.Marshal(msg)
	if err != nil {
		logging.S().Errorw("could not write result", "err", err)
		return
	}

	tgw.Lock()
	defer tgw.Unlock()

	_, err = tgw.out.Write(json)
	if err != nil {
		logging.S().Errorw("could not write result", "err", err)
	}
}

func (tgw *TgWriter) WriteError(message string, keysAndValues ...interface{}) {
	tgw.Warnw(message, keysAndValues...)

	if len(keysAndValues) > 0 {
		b := &strings.Builder{}
		for i := 0; i < len(keysAndValues); i = i + 2 {
			fmt.Fprintf(b, "%s: %s;", keysAndValues[i], keysAndValues[i+1])
		}
		kvs := b.String()
		message = message + "; " + kvs[:len(kvs)-1]
	}

	pld := Msg{Type: "error", Error: &Error{message}}
	json, err := json.Marshal(pld)
	if err != nil {
		logging.S().Errorw("could not write error response", "err", err)
		return
	}

	tgw.Lock()
	defer tgw.Unlock()

	_, err = tgw.out.Write(json)
	if err != nil {
		logging.S().Errorw("could not write error response", "err", err)
	}
}

func (tgw *TgWriter) Flush() {
	if f, ok := tgw.out.(http.Flusher); ok {
		f.Flush()
	}
}

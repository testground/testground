package rpctest

import (
	"net/http/httptest"
	"strings"

	"github.com/testground/testground/pkg/rpc"
)

// NewRecordedOutputWriter returns an OutputWriter where the response is recorded.
func NewRecordedOutputWriter(req_id string) (rec *httptest.ResponseRecorder, ow *rpc.OutputWriter) {
	req := httptest.NewRequest("GET", "/", strings.NewReader(""))
	req.Header.Add("X-Request-ID", req_id)
	rec = httptest.NewRecorder()
	ow = rpc.NewOutputWriter(rec, req)
	return rec, ow
}

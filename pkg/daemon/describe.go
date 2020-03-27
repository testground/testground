package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/client"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"
)

var TermExplanation = "a term is any of: <testplan> or <testplan>/<testcase>"

func (srv *Daemon) describeHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logging.S().With("ruid", r.Header.Get("X-Request-ID"))

		log.Debugw("handle request", "command", "describe")
		defer log.Debugw("request handled", "command", "describe")

		tgw := tgwriter.New(w, r)

		var req client.DescribeRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tgw.WriteError("cannot json decode request body", "err", err)
			return
		}

		term := req.Term

		var pl, tc string
		switch splt := strings.Split(term, "/"); len(splt) {
		case 2:
			pl, tc = splt[0], splt[1]
		case 1:
			pl = splt[0]
		default:
			tgw.WriteError("unrecognized format for term", "explanation", TermExplanation)
			return
		}

		plan := engine.TestCensus().PlanByName(pl)
		if plan == nil {
			tgw.WriteError(fmt.Sprintf("plan not found, name: %s ; term: %s", pl, term))
			return
		}

		var cases []*api.TestCase
		if tc == "" {
			cases = plan.TestCases
		} else if _, tcbn, ok := plan.TestCaseByName(tc); ok {
			cases = []*api.TestCase{tcbn}
		} else {
			tgw.WriteError(fmt.Sprintf("test case not found: %s", tc))
			return
		}

		var sb strings.Builder
		plan.Describe(&sb)
		sb.WriteString("TEST CASES:\n----------\n----------\n")

		for _, tc := range cases {
			tc.Describe(&sb)
		}

		tgw.WriteResult(sb.String())
	}
}

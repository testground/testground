// TODO: find a better name, this package is required to prevent cyclic dependencies.
//  It's used to provide functions and tool that "connects" runners, tasks, etc.
package data

import (
	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/runner"
)

func DecodeResult(result interface{}) *runner.Result {
	r := &runner.Result{}
	err := mapstructure.Decode(result, r)
	if err != nil {
		logging.S().Errorw("error while decoding result", "err", err)
	}
	return r
}

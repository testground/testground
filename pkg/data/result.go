// TODO: find a better name, this package is required to prevent cyclic dependencies.
//  It's used to provide functions and tool that "connects" runners, tasks, etc.
//  Since task is a generic wire format, and runners and builder have specific implementation,
//   we need this module to interpret task result with run/builder specific implementation.
// TODO: rethink the genericity model so that this module becomes unnecessary.
package data

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
)

func DecodeRunnerResult(result interface{}) *runner.Result {
	r := &runner.Result{
		Outcome: task.OutcomeSuccess,
	}
	err := mapstructure.Decode(result, r)
	if err != nil {
		logging.S().Errorw("error while decoding result", "err", err)
	}
	return r
}

func DecodeTaskOutcome(t *task.Task) (task.Outcome, error) {
	switch t.State().State {
	case task.StateCanceled:
		return task.OutcomeCanceled, nil
	case task.StateProcessing:
		return task.OutcomeUnknown, nil
	case task.StateScheduled:
		return task.OutcomeUnknown, nil
	case task.StateComplete:
		// continue
	default:
		return "", fmt.Errorf("unexpected task state: %s", t.State().State)
	}

	switch t.Type {
	case task.TypeBuild:
		// As of today a build that completed is successful. No need to check the result.
		return task.OutcomeSuccess, nil
	case task.TypeRun:
		return DecodeRunnerResult(t.Result).Outcome, nil
	default:
		return "", fmt.Errorf("unexpected task type: %s", t.Type)
	}
}

func IsTaskOutcomeInError(t *task.Task) error {
	outcome, err := DecodeTaskOutcome(t)

	if err != nil {
		return err
	}

	if (!IsOutcomeSuccess(outcome)) {
		return fmt.Errorf("run outcome: %s", outcome)
	}

	return nil
}

func IsOutcomeSuccess(outcome task.Outcome) bool {
	return outcome == task.OutcomeSuccess
}
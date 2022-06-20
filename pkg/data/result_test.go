package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
)

func successState() []task.DatedState {
	return []task.DatedState{
		{
			State:   task.StateComplete,
			Created: time.Now(),
		},
	}
}

func TestDecodeResult(t *testing.T) {
	result1 := &runner.Result{
		Outcome:  task.OutcomeUnknown,
		Outcomes: make(map[string]*runner.GroupOutcome),
		Journal: &runner.Journal{
			Events:       make(map[string]string),
			PodsStatuses: make(map[string]struct{}),
		},
	}
	r1 := DecodeRunnerResult(result1)
	assert.NotNil(t, r1)

	result2 := [2]string{"artifact", "artifact2"}
	r2 := DecodeRunnerResult(result2)
	assert.NotNil(t, r2)
}

func TestDecodeTaskOutcomeWithGenericRunerAndUnknownOutcome(t *testing.T) {
	tested := &task.Task{
		Type:   task.TypeRun,
		States: successState(),
		Result: &runner.Result{
			Outcome: task.OutcomeUnknown,
		},
	}
	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeUnknown, r)
	assert.Nil(t, e)
}

func TestDecodeTaskOutcomeWithGenericRunerAndSuccessOutcome(t *testing.T) {

	tested := &task.Task{
		Type:   task.TypeRun,
		States: successState(),
		Result: &runner.Result{
			Outcome: task.OutcomeSuccess,
		},
	}
	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)
}

func TestDecodeTaskOutcomeWithBuilder(t *testing.T) {
	// Run with builder type => always a success
	tested := &task.Task{
		Type:   task.TypeBuild,
		States: successState(),
		Result: []string{"artfact", "artifact2"},
	}

	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)
}

func TestDecodeTaskOutcomeWithUnknownBuilder(t *testing.T) {
	// Run with unknown builder => failure
	tested := &task.Task{
		Type:   "some-name",
		States: successState(),
	}

	_, e := DecodeTaskOutcome(tested)
	assert.NotNil(t, e)
}

func TestDecodeTaskOutcomeWithCanceledState(t *testing.T) {
	// Run with state canceled => canceled outcome
	tested := &task.Task{
		Type: task.TypeRun,
		States: []task.DatedState{
			{
				State:   task.StateCanceled,
				Created: time.Now(),
			},
		},
		Result: &runner.Result{
			Outcome: task.OutcomeSuccess,
		},
	}

	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeCanceled, r)
	assert.Nil(t, e)
}

func TestDecodeTaskOutcomeWithLocalExecRunner(t *testing.T) {
	// Run with local exec runner => the result is nil, we assume outcome is Success if the task suceeded.
	tested := &task.Task{
		Type:   task.TypeRun,
		States: successState(),
		// runner outputs something like: `&api.RunOutput{RunID: input.RunID}`
		Result: nil,
	}

	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)
}

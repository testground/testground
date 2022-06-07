package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testground/testground/pkg/runner"
	"github.com/testground/testground/pkg/task"
)

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

func TestDecodeTaskOutcome(t *testing.T) {
	success_state := []task.DatedState{
		{
			State:   task.StateComplete,
			Created: time.Now(),
		},
	}

	// Run with generic runner and unknown => unknown outcome
	tested := &task.Task{
		Type:   task.TypeRun,
		States: success_state,
		Result: &runner.Result{
			Outcome: task.OutcomeUnknown,
		},
	}
	r, e := DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeUnknown, r)
	assert.Nil(t, e)

	// Run with generic runner and success => success outcome
	tested = &task.Task{
		Type:   task.TypeRun,
		States: success_state,
		Result: &runner.Result{
			Outcome: task.OutcomeSuccess,
		},
	}
	r, e = DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)

	// Run with builder type => always a success
	tested = &task.Task{
		Type:   task.TypeBuild,
		States: success_state,
		Result: []string{"artfact", "artifact2"},
	}

	r, e = DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)

	// Run with unknown builder => failure
	tested = &task.Task{
		Type:   "some-name",
		States: success_state,
	}

	_, e = DecodeTaskOutcome(tested)
	assert.NotNil(t, e)

	// Run with state cancelled => cancelled outcome
	tested = &task.Task{
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

	r, e = DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeCanceled, r)
	assert.Nil(t, e)

	// Run with local exec runner => the result is nil, we assume outcome is Success by default.
	tested = &task.Task{
		Type: task.TypeRun,
		States: []task.DatedState{
			{
				State:   task.StateCanceled,
				Created: time.Now(),
			},
		},
		// runner outputs something like: `&api.RunOutput{RunID: input.RunID}`
		Result: nil,
	}

	r, e = DecodeTaskOutcome(tested)
	assert.Equal(t, task.OutcomeSuccess, r)
	assert.Nil(t, e)
}

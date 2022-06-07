package data

import (
	"testing"

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
	task1 := &task.Task{
		Type:   task.TypeBuild,
		Result: [2]string{"artfact", "artifact2"},
	}

	r1 := DecodeTaskOutcome(task1)
	assert.Equal(t, task.OutcomeSuccess, r1)
}

package runner

import (
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/task"
)

type Result struct {
	Outcome  task.Outcome             `json:"outcome"`
	Outcomes map[string]*GroupOutcome `json:"outcomes"`
	Journal  *Journal                 `json:"journal"`
}

func newResult(input *api.RunInput) *Result {
	result := &Result{
		Outcome:  task.OutcomeUnknown,
		Outcomes: make(map[string]*GroupOutcome),
		Journal: &Journal{
			Events:       make(map[string]string),
			PodsStatuses: make(map[string]struct{}),
		},
	}

	for _, g := range input.Groups {
		result.Outcomes[g.ID] = &GroupOutcome{
			Total: g.Instances,
			Ok:    0,
		}
	}

	return result
}

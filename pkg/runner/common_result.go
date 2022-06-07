package runner

import (
	"github.com/testground/testground/pkg/task"
)

type Result struct {
	Outcome  task.Outcome             `json:"outcome"`
	Outcomes map[string]*GroupOutcome `json:"outcomes"`
	Journal  *Journal                 `json:"journal"`
}

func newResult() *Result {
	return &Result{
		Outcome:  task.OutcomeUnknown,
		Outcomes: make(map[string]*GroupOutcome),
		Journal: &Journal{
			Events:       make(map[string]string),
			PodsStatuses: make(map[string]struct{}),
		},
	}
}

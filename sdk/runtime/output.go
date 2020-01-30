package runtime

import (
	"encoding/json"
	"fmt"
	"time"
)

type Outcome string

const (
	OutcomeOK      = Outcome("ok")
	OutcomeAborted = Outcome("aborted")
	OutcomeCrashed = Outcome("crashed")
)

type MetricDefinition struct {
	Name           string `json:"name"`
	Unit           string `json:"unit"`
	ImprovementDir int    `json:"improve_dir"`
}

type Metric struct {
	*MetricDefinition

	Value float64 `json:"value"`
}

type Event struct {
	RunEnv    *RunEnv `json:"runenv"`
	Timestamp int64   `json:"timestamp"`
	Metric    *Metric `json:"metric,omitempty"`
	Result    *Result `json:"result,omitempty"`
	Message   string  `json:"msg,omitempty"`
}

type Result struct {
	Outcome Outcome `json:"outcome"`
	Reason  string  `json:"reason,omitempty"`
	Stack   string  `json:"stack,omitempty"`
}

// Message prints out an informational message.
func (r *RunEnv) Message(msg string, a ...interface{}) {
	if len(a) > 0 {
		msg = fmt.Sprintf(msg, a...)
	}
	evt := &Event{
		RunEnv:    r,
		Timestamp: time.Now().UnixNano(),
		Message:   msg,
	}

	bytes, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

// EmitMetric outputs a metric event associated with the provided metric
// definition, giving it value `value`.
func (r *RunEnv) EmitMetric(def *MetricDefinition, value float64) {
	evt := &Event{
		RunEnv:    r,
		Timestamp: time.Now().UnixNano(),
		Metric:    &Metric{def, value},
	}

	bytes, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

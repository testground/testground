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
}

// Abort outputs an abortion event, where reason is an object that will be
// output as a message by stringing it.
func (r *RunEnv) Abort(reason interface{}) {
	evt := &Event{
		RunEnv:    r,
		Timestamp: time.Now().UnixNano(),
		Result:    &Result{OutcomeAborted, fmt.Sprintf("%s", reason)},
	}

	bytes, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

// OK outputs an OK event for this test execution.
func (r *RunEnv) OK() {
	evt := &Event{
		RunEnv:    r,
		Timestamp: time.Now().UnixNano(),
		Result:    &Result{OutcomeOK, ""},
	}

	bytes, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
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

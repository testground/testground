package runtime

import (
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	EventType    string
	EventOutcome string
)

const (
	EventTypeStart   = EventType("start")
	EventTypeMessage = EventType("message")
	EventTypeMetric  = EventType("metric")
	EventTypeFinish  = EventType("finish")

	EventOutcomeOK      = EventOutcome("ok")
	EventOutcomeFailed  = EventOutcome("failed")
	EventOutcomeCrashed = EventOutcome("crashed")
)

type Event struct {
	Type       EventType    `json:"type"`
	Outcome    EventOutcome `json:"outcome,omitempty"`
	Error      string       `json:"error,omitempty"`
	Stacktrace string       `json:"stacktrace,omitempty"`
	Message    string       `json:"message,omitempty"`
	Metric     *MetricValue `json:"metric,omitempty"`
	Runenv     *RunParams   `json:"runenv,omitempty"`
}

type MetricDefinition struct {
	Name           string `json:"name"`
	Unit           string `json:"unit"`
	ImprovementDir int    `json:"dir"`
}

type MetricValue struct {
	MetricDefinition
	Value float64 `json:"value"`
}

func (e Event) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", string(e.Type))

	if e.Outcome != "" {
		oe.AddString("outcome", string(e.Outcome))
	}
	if e.Error != "" {
		oe.AddString("error", e.Error)
	}
	if e.Stacktrace != "" {
		oe.AddString("stacktrace", e.Stacktrace)
	}
	if e.Message != "" {
		oe.AddString("message", e.Message)
	}
	if e.Metric != nil {
		if err := oe.AddObject("metric", e.Metric); err != nil {
			return err
		}
	}
	if e.Runenv != nil {
		if err := oe.AddObject("runenv", e.Runenv); err != nil {
			return err
		}
	}

	return nil
}

func (m MetricValue) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	if m.Name == "" {
		return nil
	}
	oe.AddString("name", m.Name)
	oe.AddString("unit", m.Unit)
	oe.AddInt("dir", m.ImprovementDir)
	oe.AddFloat64("value", m.Value)
	return nil
}

func (r *RunParams) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("plan", r.TestPlan)
	oe.AddString("case", r.TestCase)
	oe.AddInt("seq", r.TestCaseSeq)
	if err := oe.AddReflected("params", r.TestInstanceParams); err != nil {
		return err
	}
	oe.AddInt("instances", r.TestInstanceCount)
	oe.AddString("outputs_path", r.TestOutputsPath)
	oe.AddString("network", func() string {
		if r.TestSubnet != nil {
			return ""
		}
		return r.TestSubnet.Network()
	}())

	oe.AddString("group", r.TestGroupID)
	oe.AddInt("group_instances", r.TestGroupInstanceCount)

	if r.TestRepo != "" {
		oe.AddString("repo", r.TestRepo)
	}
	if r.TestCommit != "" {
		oe.AddString("commit", r.TestCommit)
	}
	if r.TestBranch != "" {
		oe.AddString("branch", r.TestBranch)
	}
	if r.TestTag != "" {
		oe.AddString("tag", r.TestTag)
	}
	return nil
}

// RecordMessage records an informational message.
func (l *logger) RecordMessage(msg string, a ...interface{}) {
	if len(a) > 0 {
		msg = fmt.Sprintf(msg, a...)
	}
	evt := Event{
		Type:    EventTypeMessage,
		Message: msg,
	}
	l.logger.Info("", zap.Object("event", evt))
	_ = l.logger.Sync()
}

func (l *logger) RecordStart() {
	evt := Event{
		Type:   EventTypeStart,
		Runenv: l.runenv,
	}

	l.logger.Info("", zap.Object("event", evt))
}

// RecordSuccess records that the calling instance succeeded.
func (l *logger) RecordSuccess() {
	evt := Event{
		Type:    EventTypeFinish,
		Outcome: EventOutcomeOK,
	}
	l.logger.Info("", zap.Object("event", evt))
}

// RecordFailure records that the calling instance failed with the supplied
// error.
func (l *logger) RecordFailure(err error) {
	evt := Event{
		Type:    EventTypeFinish,
		Outcome: EventOutcomeFailed,
		Error:   err.Error(),
	}
	l.logger.Info("", zap.Object("event", evt))
}

// RecordCrash records that the calling instance crashed/panicked with the
// supplied error.
func (l *logger) RecordCrash(err interface{}) {
	evt := Event{
		Type:       EventTypeFinish,
		Outcome:    EventOutcomeFailed,
		Error:      fmt.Sprintf("%s", err),
		Stacktrace: string(debug.Stack()),
	}
	l.logger.Error("", zap.Object("event", evt))
}

// RecordMetric records a metric event associated with the provided metric
// definition, giving it value `value`.
func (l *logger) RecordMetric(metric *MetricDefinition, value float64) {
	evt := Event{
		Type: EventTypeMetric,
		Metric: &MetricValue{
			MetricDefinition: *metric,
			Value:            value,
		},
	}
	l.logger.Info("", zap.Object("event", evt))
}

// Message prints out an informational message.
//
// Deprecated: use RecordMessage.
func (r *RunEnv) Message(msg string, a ...interface{}) {
	r.RecordMessage(msg, a...)
}

// EmitMetric outputs a metric event associated with the provided metric
// definition, giving it value `value`.
//
// Deprecated: use RecordMetric.
func (r *RunEnv) EmitMetric(metric *MetricDefinition, value float64) {
	r.RecordMetric(metric, value)
}

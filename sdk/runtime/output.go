package runtime

import (
	"fmt"

	"go.uber.org/zap"
)

var Types = struct{ Start, Message, Metric, Finish zap.Field }{
	Start:   zap.String("type", "start"),
	Message: zap.String("type", "message"),
	Metric:  zap.String("type", "metric"),
	Finish:  zap.String("type", "finish"),
}

var Outcomes = struct{ Success, Failure, Crash zap.Field }{
	Success: zap.String("outcome", "success"),
	Failure: zap.String("outcome", "failure"),
	Crash:   zap.String("outcome", "crash"),
}

type MetricDefinition struct {
	Name           string
	Unit           string
	ImprovementDir int
}

// RecordMessage records an informational message.
func (l *logger) RecordMessage(msg string, a ...interface{}) {
	if len(a) > 0 {
		msg = fmt.Sprintf(msg, a...)
	}
	l.logger.Info(msg, Types.Message)
}

func (l *logger) RecordStart() {
	l.logger.Info("",
		Types.Start,
		zap.String("plan", l.runenv.TestPlan),
		zap.String("case", l.runenv.TestCase),
		zap.String("run", l.runenv.TestRun),
		zap.Int("seq", l.runenv.TestCaseSeq),
		zap.String("repo", l.runenv.TestRepo),
		zap.String("commit", l.runenv.TestCommit),
		zap.String("branch", l.runenv.TestBranch),
		zap.String("tag", l.runenv.TestTag),
		zap.Int("instances", l.runenv.TestInstanceCount),
		zap.String("group", l.runenv.TestGroupID),
	)
}

// RecordSuccess records that the calling instance succeeded.
func (l *logger) RecordSuccess() {
	l.logger.Info("",
		Types.Finish,
		Outcomes.Success,
	)
}

// RecordFailure records that the calling instance failed with the supplied
// error.
func (l *logger) RecordFailure(err error) {
	l.logger.Error("",
		Types.Finish,
		Outcomes.Failure,
		zap.Error(err),
	)
}

// RecordCrash records that the calling instance crashed/panicked with the
// supplied error.
func (l *logger) RecordCrash(err interface{}) {
	l.logger.Error("",
		Types.Finish,
		Outcomes.Crash,
		zap.Any("error", err),
		zap.Stack("stacktrace"),
	)
}

// RecordMetric records a metric event associated with the provided metric
// definition, giving it value `value`.
func (l *logger) RecordMetric(metric *MetricDefinition, value float64) {
	l.logger.Info("",
		Types.Metric,
		zap.String("name", metric.Name),
		zap.String("unit", metric.Unit),
		zap.Float64("value", value),
	)
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

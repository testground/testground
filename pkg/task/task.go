package task

import "time"

// TaskState (kind: int) represents the last known state of a task.
// A task can be in one of three states
// StateScheduled: this is the initial state of the task when it enters the queue.
// StateProcessing: once work begins on the task, it is put into this state.
// StateComplete: work is no longer being done on this task. client should check task result.
type TaskState int

const (
	StateScheduled TaskState = iota
	StateProcessing
	StateComplete
)

func (t TaskState) String() string {
	return [...]string{
		"StateRequested",
		"StateProcessing",
		"StateComplete",
	}[t]
}

// ResultStatus (kind: int) is a status code for completed tasks.
// ResultNone: initial status, No status, probably becasue the task is incomplete.
// ResultSuccess: the task has completed without an error.
// ResultPlanFail: the task has completed with a failure.
// ResultTestgroundFail: testground encountered an error and the task has not been scheduled.
type TaskResultStatus int

const (
	ResultNone TaskResultStatus = iota
	ResultSuccess
	ResultPlanFail
	ResultTestgroundFail
)

func (t TaskResultStatus) String() string {
	return [...]string{
		"ResultNone",
		"ResultSuccess",
		"ResultPlanFail",
		"ResultTestgroundFail",
	}[t]
}

// DatedTaskState (kind: struct) is a TaskState with a timestamp.
type DatedTaskState struct {
	TaskState TaskState `json:"state"`
	Entered   time.Time `json:"entered"`
}

// TaskResult (kind: struct)  contains a status code. If the status is not TaskResultNone or
// TaskResultSuccess, relevant errors will be included in this struct.
type TaskResult struct {
	Status TaskResultStatus `json:"status"`
	Errors []error          `json:"errors"`
}

// Task (kind: struct) contains metadata about a testground task. This schema is used to store
// metadata in our task storage database as well as the wire format returned when clients get the
// state of a running or secheduled task.
type Task struct {
	Version  int              `json:"version"`  // Schema version
	Priority int              `json:"priority"` // scheduling priority
	Created  time.Time        `json:"created"`  // datetime created
	ID       string           `json:"id"`       // unique identifier for this task
	States   []DatedTaskState `json:"states"`   // State of the task
	Result   TaskResult       `json:"result"`   // result of the task, when terminal.
}

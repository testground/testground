package task

import "github.com/google/uuid"
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

// TaskType(kind int) represents the kind of activity the daemon asked to perform. In alignment
// with the testground command-line we have two kinds of tasks
// TaskBuild -- which functions similarly to `testground build`. The result of this task will contain
// a build ID which can be used in a subsiquent run.
// TaskRun -- which functions similarly to `testground run`
type TaskType int

const (
	TaskBuild TaskType = iota
	TaskRun
)

// DatedTaskState (kind: struct) is a TaskState with a timestamp.
type DatedTaskState struct {
	Created   time.Time `json:"created"`
	TaskState TaskState `json:"state"`
}

// TaskResult (kind: struct)
// This will be redefined at a later time.
type TaskResult struct{}

// Task (kind: struct) contains metadata about a testground task. This schema is used to store
// metadata in our task storage database as well as the wire format returned when clients get the
// state of a running or secheduled task.
type Task struct {
	Version  int              `json:"version"`  // Schema version
	Priority int              `json:"priority"` // scheduling priority
	ID       string           `json:"id"`       // unique identifier for this task, specifically, a UUID
	States   []DatedTaskState `json:"states"`   // State of the task
	Result   TaskResult       `json:"result"`   // result of the task, when terminal.
}

func (t *Task) Created() time.Time {
	u, err := uuid.Parse(t.ID)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(u.Time().UnixTime())
}

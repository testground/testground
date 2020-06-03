package task

import "time"

// TaskState (kind: int) represents the last known state of a task.
// A task can be in one of three states
// TaskStateScheduled: this is the initial state of the task when it enters the queue.
// TaskStateProcessing: once work begins on the task, it is put into this state.
// TaskStateComplete: work is no longer being done on this task. client should check task result.
type TaskState int

const (
	TaskStateScheduled TaskState = iota
	TaskStateProcessing
	TaskStateComplete
)

func (t TaskState) String() string {
	return [...]string{
		"TaskStateRequested",
		"TaskStateProcessing",
		"TaskStateComplete",
	}[t]
}

// TaskResultStatus (kind: int) is a status code for completed tasks.
// TaskResultNone: initial status, No status, probably becasue the task is incomplete.
// TaskResultSuccess: the task has completed without an error.
// TaskResultFail: the task has completed with a failure.
// TaskResultAbort: testground encountered an error and the task has not been scheduled.
// testground will respond to client with a TaskResultAbort status if it is unable to enqueue
// the reqeusted task.
type TaskResultStatus int

const (
	TaskResultNone TaskResultStatus = iota
	TaskResultSuccess
	TaskResultFail
	TaskResultAbort
)

func (t TaskResultStatus) String() string {
	return [...]string{
		"TaskResultNone",
		"TaskResultSuccess",
		"TaskResultFail",
		"TaskResultAbort",
	}[t]
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
	Version  int        `json:"version"`  // Schema version
	Priority int        `json:"priority"` // scheduling priority
	Created  time.Time  `json:"created"`  // datetime created
	ID       string     `json:"id"`       // unique identifier for this task
	State    TaskState  `json:"state"`    // State of the task
	Result   TaskResult `json:"result"`   // result of the task, when terminal.
}

// This is a priority queue which implements container/heap.Interface
// Tasks are sorted by priority and then timestamp.
type TaskQueue []*Task

func (q TaskQueue) Len() int {
	return len(q)
}

func (q TaskQueue) Less(i, j int) bool {
	if q[i].Priority != q[j].Priority {
		return q[i].Priority > q[j].Priority
	}
	return q[i].Created.Before(q[j].Created)
}

func (q TaskQueue) Swap(i, j int) {
	q[j], q[i] = q[i], q[j]
}

func (q *TaskQueue) Push(x interface{}) {
	t := x.(*Task)
	*q = append(*q, t)
}

func (q *TaskQueue) Pop() interface{} {
	t := (*q)[len(*q)-1]
	*q = (*q)[:len(*q)-1]
	return t
}

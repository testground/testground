package daemon

import "time"

const (
	TaskStateRequested TaskState = iota // when client requests a task
	TaskStateScheduled                  // daemon accepts the task into the queue
	TaskStateBuilding                   // daemon is building the task
	TaskStateRunning                    // daemon is running the task
	TaskStateComplete                   // daemon is not running the task. ready to collect.
)

type TaskState int

func (t TaskState) String() string {
	return [...]string{
		"requested",
		"scheduled",
		"building",
		"running",
		"complete",
	}[t]
}

// Keep this struct simple. This structure is persited to the datastore.
type Task struct {
	Priority int       `json:"priority"`
	Created  time.Time `json:"created"`
	ID       string    `json:"id"`
	State    TaskState `json:"state"`
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

package task

import (
	"container/heap"
	"encoding/json"
	"errors"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	ErrQueueEmpty = errors.New("queue empty")
	ErrQueueFull  = errors.New("queue full")
)

func NewQueue(ts *TaskStorage, max int) (*Queue, error) {
	tq := new(taskQueue)
	// read the active tasks into the queue
	iter := ts.db.NewIterator(util.BytesPrefix([]byte(QUEUEPREFIX)), nil)
	for iter.Next() {
		tsk := new(Task)
		err := json.Unmarshal(iter.Value(), tsk)
		if err != nil {
			return nil, err
		}
		// If the current state is Scheduled, we need to place it into the queue.
		ln := len(tsk.States)
		if ln == 0 || tsk.States[ln-1].TaskState == StateScheduled {
			heap.Push(tq, tsk)
		}
	}
	iter.Release()
	// correct the eviction order so we will evict oldest items first
	return &Queue{
		tq:  tq,
		ts:  ts,
		max: max,
	}, nil
}

// Queue is a priority queue for tasks.
type Queue struct {
	sync.Mutex
	tq *taskQueue
	ts *TaskStorage

	max int // the maximum number of tasks to keep in the databse
}

// Add an item to the priority queue
// 1. Check if we have too many items enqueued already.
// 2. Persist task to the database.
// 3. Push the task into the in-memory heap.
func (q *Queue) Push(tsk *Task) error {
	q.Lock()
	defer q.Unlock()

	// there are too many items enqueued already. can't push; try again later.
	if q.tq.Len() >= q.max {
		return ErrQueueFull
	}

	// Persist this task to the database
	err := q.ts.Put(QUEUEPREFIX, tsk)
	if err != nil {
		return err
	}
	// Push this task to the queue
	heap.Push(q.tq, tsk)
	return nil
}

// get the next item from the priority queue
// Pop the task off of the queue
// The task remains in the database, but is no longer in the heap.
// As the state of the task changes (i.e. to mark the task completed, use SetTaskState)
func (q *Queue) Pop() (*Task, error) {
	q.Lock()
	defer q.Unlock()
	if q.tq.Len() == 0 {
		return nil, ErrQueueEmpty
	}
	tsk := heap.Pop(q.tq).(*Task)

	q.ts.Put(CURRENTPREFIX, tsk)
	return tsk, nil
}

// This is a priority queue which implements container/heap.Interface
// Tasks are sorted by priority and then timestamp.
type taskQueue []*Task

func (q taskQueue) Len() int {
	return len(q)
}

func (q taskQueue) Less(i, j int) bool {
	if q[i].Priority != q[j].Priority {
		return q[i].Priority > q[j].Priority
	}
	return q[i].Created.Before(q[j].Created)
}

func (q taskQueue) Swap(i, j int) {
	q[j], q[i] = q[i], q[j]
}

func (q *taskQueue) Push(x interface{}) {
	t := x.(*Task)
	*q = append(*q, t)
}

func (q *taskQueue) Pop() interface{} {
	t := (*q)[len(*q)-1]
	*q = (*q)[:len(*q)-1]
	return t
}

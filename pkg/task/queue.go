package task

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/testground/testground/pkg/logging"
)

var (
	ErrQueueEmpty = errors.New("queue empty")
	ErrQueueFull  = errors.New("queue full")
)

func NewQueue(ts *Storage, max int, converter func([]byte) (*Task, error)) (*Queue, error) {
	tq := new(taskQueue)
	for _, prefix := range []string{prefixScheduled, prefixProcessing} {
		// read the active tasks into the queue
		iter := ts.db.NewIterator(util.BytesPrefix([]byte(prefix)), nil)
		for iter.Next() {
			tsk, err := converter(iter.Value())
			if err != nil {
				return nil, err
			}
			heap.Push(tq, tsk)
		}
		iter.Release()
	}
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
	ts *Storage

	max int // the maximum number of tasks to keep in the database
}

// Add an item to the priority queue
// 1. Check if we have too many items enqueued already.
// 2. Persist task to the database.
// 3. Push the task into the in-memory heap.
func (q *Queue) Push(tsk *Task) error {
	q.Lock()
	defer q.Unlock()

	return q.pushLocked(tsk)
}

// Pushes an item to the priority queue, without acquiring a lock
func (q *Queue) pushLocked(tsk *Task) error {
	// there are too many items enqueued already. can't push; try again later.
	if q.tq.Len() >= q.max {
		return ErrQueueFull
	}

	// Persist this task to the database
	logging.S().Debugw("queue.push.got-task", "id", tsk.ID, "taskname", tsk.Name())
	err := q.ts.PersistScheduled(tsk)
	if err != nil {
		return err
	}
	// Push this task to the queue
	heap.Push(q.tq, tsk)

	return nil
}

// Pushes a task to the queue, and removes any tasks with matching repo/branch from the queue
func (q *Queue) PushUniqueByBranch(tsk *Task) error {
	q.Lock()
	defer q.Unlock()

	// Remove existing tasks from same branch end repo before pushing a new task
	var err error
	if tsk.CreatedBy.Repo != "" && tsk.CreatedBy.Branch != "" {
		err = q.RemoveExisting(tsk.CreatedBy.Branch, tsk.CreatedBy.Repo)
	}

	if err != nil {
		return err
	}

	err = q.pushLocked(tsk)

	return err
}

// get the next item from the priority queue
// Pop the task off of the queue
// The task remains in the database, but is no longer in the heap.
// As the state of the task changes
func (q *Queue) Pop() (*Task, error) {
	q.Lock()
	defer q.Unlock()
	if q.tq.Len() == 0 {
		return nil, ErrQueueEmpty
	}
	logging.S().Debugw("queue.pop", "len", q.tq.Len())
	tsk := heap.Pop(q.tq).(*Task)

	logging.S().Debugw("queue.pop.got-task", "id", tsk.ID, "taskname", tsk.Name())
	err := q.ts.ProcessTask(tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

// Remove all existing tasks from the queue that match the given branch/string
func (q *Queue) RemoveExisting(branch string, repo string) error {
	var err error
	keep_indexes := make([]int, 0)
	for index, qTask := range *q.tq {
		// if task matches both branch and repo, cancel it
		if qTask.CreatedBy.Repo == repo && qTask.CreatedBy.Branch == branch {
			err = q.cancelTask(qTask)
			if err != nil {
				return err
			}

		} else {
			keep_indexes = append(keep_indexes, index)
		}
	}
	keep_tasks := make([]*Task, len(keep_indexes))
	for index, value := range keep_indexes {
		keep_tasks[index] = (*q.tq)[value]
	}

	*q.tq = keep_tasks

	return nil
}

// Cancels the given task:
// 1. Changes the state to Canceled
// 2. Persists changes to the queue storage
func (q *Queue) cancelTask(tsk *Task) error {
	var err error

	// Move task to "processing" state
	err = q.ts.ProcessTask(tsk)
	if err != nil {
		return err
	}

	newState := DatedState{
		Created: time.Now().UTC(),
		State:   StateCanceled,
	}
	tsk.States = append(tsk.States, newState)
	// Apply state changes
	err = q.ts.PersistProcessing(tsk)
	if err != nil {
		return err
	}

	// Move task to "archived" state
	err = q.ts.ArchiveTask(tsk)
	return err
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

	// This will silently work incorrectly! using default time.Time{} will cause the queue to be
	// mis-sorted among tasks of the same priority.

	return q[i].Created().Before(q[j].Created())
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

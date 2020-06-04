package task

import (
	"container/heap"
	"encoding/json"
	"errors"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

var (
	ErrQueueEmpty = errors.New("empty queue")
	ErrQueueFull  = errors.New("queue full")
)

func initQueue(s storage.Storage, max int) (*Queue, error) {
	db, err := leveldb.Open(s, nil)
	if err != nil {
		return nil, err
	}
	tq := new(taskQueue)
	// read the active tasks into the queue
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		tsk := new(Task)
		err := json.Unmarshal(iter.Value(), tsk)
		if err != nil {
			return nil, err
		}
		// In the future, perform schema migration if necessary
		if tsk.State != StateComplete {
			heap.Push(tq, tsk)
		}
	}
	iter.Release()
	return &Queue{
		db:  db,
		tq:  tq,
		max: max,
	}, nil
}

func NewPersistentQueue(max int, path string) (*Queue, error) {
	s, err := storage.OpenFile(path, false)
	if err != nil {
		return nil, err
	}
	return initQueue(s, max)
}

func NewInmemQueue(max int) (*Queue, error) {
	s := storage.NewMemStorage()
	return initQueue(s, max)
}

// Queue is a priority queue for tasks which uses a key-value databse for persistence.
// It consists of a heap of Tasks and a leveldb backend.
type Queue struct {
	sync.Mutex
	tq *taskQueue
	db *leveldb.DB

	max int
}

// Add an item to the priority queue
// 1. Persist the task to the database
// 2. Push the task onto the heap
func (q *Queue) Push(tsk *Task) error {
	q.Lock()
	defer q.Unlock()
	if q.tq.Len() >= q.max {
		return ErrQueueFull
	}
	err := q.put(tsk)
	if err != nil {
		return err
	}
	heap.Push(q.tq, tsk)
	return nil
}

// Get an Task from the K-V store
// 1. Lookup the key in the database.
// 2. Unmarshal the task into the provided task pointer.
func (q *Queue) Get(id string, tsk *Task) error {
	key := []byte(id)
	val, err := q.db.Get(key, nil)
	if err != nil {
		return err
	}
	err = json.Unmarshal(val, tsk)
	if err != nil {
		return err
	}
	return nil
}

// unexported; put value into the K-V store.
func (q *Queue) put(tsk *Task) error {
	key := []byte(tsk.ID)
	val, err := json.Marshal(tsk)
	if err != nil {
		return err
	}
	return q.db.Put(key, val, &opt.WriteOptions{
		Sync: true,
	})
}

// get the next item from the priority queue
// Pop the task off of the queue
// The task remains in the database, but is no longer in the heap.
// As the state of the task changes (i.e. to mark the task completed, use SetState)
func (q *Queue) Pop() (*Task, error) {
	q.Lock()
	defer q.Unlock()
	if q.tq.Len() == 0 {
		return nil, ErrQueueEmpty
	}
	tsk := heap.Pop(q.tq).(*Task)
	return tsk, nil
}

// delete a task from the queue.
// This method can be used to cancel an enqueued task before it is executed or remove a reference to
// a completed task.
// 1. Delete the key from the database
// 2. Remove the element from the queue, if it exists.
func (q *Queue) Delete(id string) error {
	q.Lock()
	defer q.Unlock()
	err := q.db.Delete([]byte(id), nil)
	if err != nil {
		return err
	}
	for i, t := range *q.tq {
		if t.ID == id {
			_ = heap.Remove(q.tq, i).(*Task)
			break
		}
	}
	return nil
}

// Change the state of a task in the K-V store
func (q *Queue) SetTaskState(id string, state TaskState) error {
	q.Lock()
	defer q.Unlock()
	tsk := new(Task)
	err := q.Get(id, tsk)
	if err != nil {
		return err
	}
	tsk.State = state
	if err := q.put(tsk); err != nil {
		return err
	}
	return nil
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

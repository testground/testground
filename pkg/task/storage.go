package task

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	SCHEMAVERSION int = 1
)

func NewQueue(max int, path string) (*Queue, error) {
	// open the database
	db, err := leveldb.OpenFile(path, nil)
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
		if tsk.State != TaskStateComplete {
			heap.Push(tq, tsk)
		}
	}
	iter.Release()
	return &Queue{
		tq:  tq,
		db:  db,
		max: max,
	}, nil
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
func (s *Queue) Push(tsk *Task) error {
	s.Lock()
	defer s.Unlock()
	if s.Len() >= s.max {
		return fmt.Errorf("push rejected. too many items.")
	}
	err := s.put(tsk)
	if err != nil {
		return err
	}
	heap.Push(s.tq, tsk)
	return nil
}

// Get an Task from the K-V store. The returned Task may or may not be in the heap
func (s *Queue) Get(id string) (*Task, error) {
	key := []byte(id)
	val, err := s.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	tsk := new(Task)
	err = json.Unmarshal(val, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

// unexported; put value into the K-V store.
func (s *Queue) put(tsk *Task) error {
	key := []byte(tsk.ID)
	val, err := json.Marshal(tsk)
	if err != nil {
		return err
	}
	return s.db.Put(key, val, &opt.WriteOptions{
		Sync: true,
	})
}

// get the next item from the priority queue
// 1. Mark the task in progress in the database
// 2. Pop the task off of the queue
func (s *Queue) Pop() (*Task, error) {
	s.Lock()
	defer s.Unlock()
	if s.tq.Len() == 0 {
		return nil, fmt.Errorf("attempted pop on empty queue, returning nil pointer!")
	}
	tsk := heap.Pop(s.tq).(*Task)
	tsk.State = TaskStateProcessing
	return tsk, nil
}

// delete a task from the queue
// 1. Delete the key from the database
// 2. Remove the element from the queue, if it exists.
func (s *Queue) Delete(id string) error {
	s.Lock()
	defer s.Unlock()
	err := s.db.Delete([]byte(id), nil)
	if err != nil {
		return err
	}
	for i, t := range *s.tq {
		if t.ID == id {
			_ = heap.Remove(s.tq, i).(*Task)
			break
		}
	}
	return nil
}

// Change the state of a task in the K-V store
func (s *Queue) SetTaskState(id string, state TaskState) error {
	s.Lock()
	defer s.Unlock()
	tsk, err := s.Get(id)
	if err != nil {
		return err
	}
	tsk.State = state
	if err := s.put(tsk); err != nil {
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

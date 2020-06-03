package task

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// TaskStorage is a leveldb-backed priority queue.
// storage is persisted before the in-memory queue to prevent inconsistency between restarts; what
// you see in the queue is the same as what is written the database
type TaskStorage struct {
	// required configuration
	Max  int
	Path string

	// optional configuration
	DBOpts    *opt.Options
	WriteOpts *opt.WriteOptions
	ReadOpts  *opt.ReadOptions

	// mux protects the tq. Although db is already goroutine-safe, it it is kept in sync with tq.
	mux sync.Mutex
	tq  *TaskQueue
	db  *leveldb.DB
}

// Open database and load its contents into memory.
func (s *TaskStorage) Open() error {
	db, err := leveldb.OpenFile(s.Path, s.DBOpts)
	if err != nil {
		return err
	}
	s.db = db
	return s.Reload()
}

// Method for closing the database. Always run Close when finished.
func (s *TaskStorage) Close() error {
	s.tq = nil
	return s.db.Close()
}

// Read everything from the database into memory. Typically, you will not need to run this; it is
// executed automatically when the database is opened.
func (s *TaskStorage) Reload() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.tq = new(TaskQueue)
	iter := s.db.NewIterator(nil, s.ReadOpts)
	for iter.Next() {
		tsk := new(Task)
		err := json.Unmarshal(iter.Value(), tsk)
		if err != nil {
			return err
		}
		if tsk.State != TaskStateComplete {
			heap.Push(s.tq, tsk)
		}
	}
	iter.Release()
	return iter.Error()
}

// Add an item to the priority queue
func (s *TaskStorage) Push(tsk *Task) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.Len() >= s.Max {
		return fmt.Errorf("push rejected. too many items.")
	}
	key := []byte(tsk.ID)
	val, err := json.Marshal(tsk)
	if err != nil {
		return err
	}
	err = s.db.Put(key, val, nil)
	if err != nil {
		return err
	}
	heap.Push(s.tq, tsk)
	return nil
}

// get the next item from the priority queue
func (s *TaskStorage) Pop() (*Task, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.Len() == 0 {
		return nil, fmt.Errorf("attempted pop on empty queue, returning nil pointer!")
	}
	tsk := heap.Pop(s.tq).(*Task)
	key := []byte(tsk.ID)
	err := s.db.Delete(key, nil)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

// delete a task from the queue
func (s *TaskStorage) Delete(tsk *Task) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	err := s.db.Delete([]byte(tsk.ID), s.WriteOpts)
	if err != nil {
		return err
	}
	for i, t := range *s.tq {
		if t.ID == tsk.ID {
			heap.Remove(s.tq, i)
			break
		}
	}
	return nil
}

// Change the state of a task
func (s *TaskStorage) SetState(tsk *Task, state TaskState) error {
	key := []byte(tsk.ID)
	t, err := s.Get(tsk.ID)
	if err != nil {
		return err
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	t.State = state
	newbuf, err := json.Marshal(t)
	if err != nil {
		return err
	}
	err = s.db.Put(key, newbuf, s.WriteOpts)
	if err != nil {
		return err
	}
	tsk.State = state
	return nil
}

// Get a task by key
func (s *TaskStorage) Get(key string) (*Task, error) {
	buf, err := s.db.Get([]byte(key), s.ReadOpts)
	if err != nil {
		return nil, err
	}
	tsk := new(Task)
	err = json.Unmarshal(buf, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

func (s *TaskStorage) Len() int {
	return len(*(s.tq))
}

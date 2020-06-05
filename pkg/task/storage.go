package task

import (
	"container/heap"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

var (
	ErrQueueEmpty = errors.New("queue empty")
	ErrQueueFull  = errors.New("queue full")
)

func initQueue(s storage.Storage, max int, onEvict EvictionFunction) (*Queue, error) {
	db, err := leveldb.Open(s, nil)
	if err != nil {
		return nil, err
	}
	tq := new(taskQueue)
	eo := make([]*evict, 0)
	// read the active tasks into the queue
	iter := db.NewIterator(nil, nil)
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
		eo = append(eo, &evict{
			Key:  tsk.ID,
			Time: tsk.Created,
		})
	}
	iter.Release()
	sort.Slice(eo, func(i, j int) bool {
		return eo[i].Time.Before(eo[j].Time)
	})
	return &Queue{
		tq:      tq,
		db:      db,
		eo:      eo,
		max:     max,
		onEvict: onEvict,
	}, nil
}

func NewPersistentQueue(max int, onEvict EvictionFunction, path string) (*Queue, error) {
	s, err := storage.OpenFile(path, false)
	if err != nil {
		return nil, err
	}
	return initQueue(s, max, onEvict)
}

func NewInmemQueue(max int, onEvict EvictionFunction) (*Queue, error) {
	s := storage.NewMemStorage()
	return initQueue(s, max, onEvict)
}

type Queue struct {
	sync.Mutex
	tq *taskQueue  // priority task queue
	db *leveldb.DB // on-disk key-value databse
	eo []*evict    // eviction order when there are too many keys.

	max     int
	onEvict EvictionFunction // Additional cleanup function when eviction occurs.
}

// Add an item to the priority queue
// 1. Check if there are more than the maximum allowed keys in the database
//    a. if there are, evict old keys
//    b. call eviction function
// 2. Persist the new task to the database
// 3. Push the new task onto the queue
func (q *Queue) Push(tsk *Task) error {
	q.Lock()
	defer q.Unlock()

	// special case: there are too many items enqueued already. can't push; try again later.
	if q.tq.Len() >= q.max {
		return ErrQueueFull
	}
	// evict keys from the database until we have less than the max.
	for keys := len(q.eo); keys >= q.max; keys-- {
		key := q.eo[0].Key
		err := q.db.Delete([]byte(key), &opt.WriteOptions{
			Sync: true,
		})
		if err != nil {
			return err
		}
		q.onEvict(key)
		q.eo = q.eo[1:]
	}

	// Persist this task to the database
	err := q.put(tsk)
	if err != nil {
		return err
	}
	// Add this task to the eviction order
	q.eo = append(q.eo, &evict{tsk.ID, tsk.Created})
	// Push this task to the queue
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
// As the state of the task changes (i.e. to mark the task completed, use SetTaskState)
func (q *Queue) Pop() (*Task, error) {
	q.Lock()
	defer q.Unlock()
	if q.tq.Len() == 0 {
		return nil, ErrQueueEmpty
	}
	tsk := heap.Pop(q.tq).(*Task)
	return tsk, nil
}

// Change the state of a task in the K-V store
// This method
func (q *Queue) AppendTaskState(id string, state TaskState) error {
	q.Lock()
	defer q.Unlock()
	tsk := new(Task)
	err := q.Get(id, tsk)
	if err != nil {
		return err
	}
	dated := DatedTaskState{
		TaskState: state,
		Entered:   time.Now(),
	}
	tsk.States = append(tsk.States, dated)
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

// This is used to keep track of which element to evict when the database is full.
type evict struct {
	Key  string
	Time time.Time
}

// Cleanup function, which is executed whenever an element is evicted from the database
// Use this function to delete files that exist outside of the database
type EvictionFunction func(key string)

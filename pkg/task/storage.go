package task

import (
	"container/heap"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	ErrQueueEmpty = errors.New("queue empty")
	ErrQueueFull  = errors.New("queue full")

	// database key prefixes
	QUEUEPREFIX   = "queued"
	ARCHIVEPREFIX = "archive"
)

// taskPrefix creates a ranged key prefix. The keys will look like this for a task
func taskPrefix(prefix string, tsk *Task) *util.Range {
	s := strings.Join([]string{prefix, tsk.ID}, ":")
	return util.BytesPrefix([]byte(s))
}

func initQueue(s storage.Storage, max int) (*Queue, error) {
	db, err := leveldb.Open(s, nil)
	if err != nil {
		return nil, err
	}
	tq := new(taskQueue)
	// read the active tasks into the queue
	iter := db.NewIterator(util.BytesPrefix([]byte(QUEUEPREFIX)), nil)
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
		db:  db,
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

type Queue struct {
	sync.Mutex
	tq *taskQueue  // priority task queue
	db *leveldb.DB // on-disk key-value databse

	max int // the maximum number of tasks to keep in the databse
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

	// Persist this task to the database
	err := q.put(tsk)
	if err != nil {
		return err
	}
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
	strkey := strings.Join([]string{QUEUEPREFIX, tsk.ID}, ":")
	key := []byte(strkey)
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

package task

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// Make sure items pushed into the into the TaskStorage are persisted
// Make sure they are removed from persistent storage.
func TestStorageIsPersistent(t *testing.T) {
	q, err := NewInmemQueue(1, EvictDoNothing)
	if err != nil {
		t.Fatal(err)
	}
	tsk := &Task{
		ID: "abc123",
	}
	// store a task using the TaskStorage
	err = q.Push(tsk)
	if err != nil {
		t.Fatal(err)
	}
	// read the object from the backend
	buf, err := q.db.Get([]byte("abc123"), nil)
	if err != nil {
		t.Fatal(err)
	}
	tsk2 := new(Task)
	// Should be the same
	err = json.Unmarshal(buf, tsk2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, (*tsk).ID, (*tsk2).ID)
}

// Simulate persistance between restarts.
// Push to a q1, make sure it persists when q2 has the same storage.
func TestStorageReloads(t *testing.T) {
	id := "abc123"
	/// Both queues qill use the same storage
	stor := storage.NewMemStorage()

	// open q1 and push an item into the queue
	q1, err := initQueue(stor, 1, EvictDoNothing)
	if err != nil {
		t.Fatal(err)
	}
	err = q1.Push(&Task{
		ID: id,
	})
	if err != nil {
		t.Fatal(err)
	}
	q1.db.Close() // sync and release lock

	// open q2 with the same storage
	q2, err := initQueue(stor, 1, EvictDoNothing)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the q2 has an item in it.
	assert.Equal(t, 1, q2.tq.Len())

	// Make sure it's the same item
	tsk, err := q2.Pop()
	if err != nil {
		t.Fail()
	}
	assert.Equal(t, id, tsk.ID)
}

func TestEviction(t *testing.T) {
	qmax := 5
	evicted := make([]string, 0)
	evfunc := func(key string) {
		evicted = append(evicted, key)
	}

	q, err := NewInmemQueue(qmax, evfunc)
	if err != nil {
		t.Fatal(err)
	}

	// Load up the queue
	for i := 0; i < qmax; i++ {
		err := q.Push(&Task{
			ID:      "tasknumber-" + strconv.Itoa(i),
			Created: time.Now(),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	assert.Equal(t, qmax, q.tq.Len())
	assert.Equal(t, qmax, len(q.eo))

	// The queue is now full and the database has its max keys.
	// There are no evictable keys, so we will encounter an error
	err = q.Push(&Task{})
	assert.EqualError(t, err, ErrQueueFull.Error())

	// pop an element off of the task queue.
	_, err = q.Pop()
	if err != nil {
		t.Fatal(err)
	}

	// this time, when we push to the queue, it will evict the oldest element.
	err = q.Push(&Task{})
	assert.Nil(t, err)
	assert.Len(t, evicted, 1)
	assert.Equal(t, "tasknumber-0", evicted[0])
}

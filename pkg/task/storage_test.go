package task

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// Make sure items pushed into the into the TaskStorage are persisted
// Make sure they are removed from persistent storage.
func TestStorageIsPersistent(t *testing.T) {
	q, err := NewInmemQueue(1)
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
	q1, err := initQueue(stor, 1)
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
	q2, err := initQueue(stor, 1)
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

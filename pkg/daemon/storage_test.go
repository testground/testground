package daemon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// Make sure items pushed into the into the TaskStorage are persisted
// Make sure they are removed from persistent storage.
func TestStorageIsPersistent(t *testing.T) {
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	stor := TaskStorage{
		Max: 1,
		db:  db,
		tq:  &TaskQueue{},
	}
	tsk := &Task{
		Priority: 1,
		ID:       "abc123",
		Created:  time.Now(),
	}
	// store a task using the TaskStorage
	err = stor.Push(tsk)
	if err != nil {
		t.Fatal(err)
	}
	// read the object from the backend
	buf, err := db.Get([]byte("abc123"), nil)
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

// Make sure data persisted to the storage can be read into the queue
func TestStorageReloads(t *testing.T) {
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	stor1 := TaskStorage{
		Max: 1,
		db:  db,
		tq:  &TaskQueue{},
	}
	tsk1 := &Task{
		Priority: 1,
		ID:       "abc123",
		Created:  time.Now(),
	}
	// store a task using the TaskStorage
	err = stor1.Push(tsk1)
	if err != nil {
		t.Fatal(err)
	}
	// like a restart, create a new TaskStorage with the same db
	stor2 := TaskStorage{
		Max: 1,
		db:  db,
		tq:  &TaskQueue{},
	}
	// Should have the items saved previously
	err = stor2.Reload()
	if err != nil {
		t.Fail()
	}
	assert.Equal(t, 1, stor2.Len())
}

package task

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// Make sure items pushed into the into the Queue are persisted
// Make sure they are removed from persistent storage.
func TestQueueIsPersistent(t *testing.T) {
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	q, err := NewQueue(&Storage{db}, 1, convertTask)
	if err != nil {
		t.Fatal(err)
	}
	tsk := &Task{
		ID: "bt4brhjpc98qra498sg0",
	}
	// store a task using the Storage
	err = q.Push(tsk)
	if err != nil {
		t.Fatal(err)
	}
	// read the object from the backend
	tsk2, err := q.ts.get(prefixScheduled, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, (*tsk).ID, (*tsk2).ID)
}

// Simulate persistance between restarts.
// Push to a q1, make sure it persists when q2 has the same storage.
func TestQueueReloads(t *testing.T) {
	id := "bt4brhjpc98qra498sg0"
	/// Both queues will use the same storage
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// open q1 and push an item into the queue
	q1, err := NewQueue(ts, 1, convertTask)
	if err != nil {
		t.Fatal(err)
	}
	err = q1.Push(&Task{
		ID: id,
	})
	if err != nil {
		t.Fatal(err)
	}

	// open q2 with the same storage
	q2, err := NewQueue(ts, 1, convertTask)
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

func TestQueueRemovesTasksPerBranch(t *testing.T) {
	/// Both queues will use the same storage
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// open q1 and push an item into the queue
	q1, err := NewQueue(ts, 100, convertTask)
	if err != nil {
		t.Fatal(err)
	}

	id := "bt4brhjpc98qra498sg0"
	branch := "test_branch"
	repo := "test_repo"
	states := []DatedState{{State: StateScheduled, Created: time.Now()}}
	err = q1.Push(&Task{
		ID: id,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo,
		},
		States: states,
	})
	if err != nil {
		t.Fatal(err)
	}

	// task with same branch, but different repo
	id2 := "bt4brhjpc98qra498sg1"
	repo2 := "another_repo"
	err = q1.Push(&Task{
		ID: id2,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo2,
		},
		States: states,
	})
	if err != nil {
		t.Fatal(err)
	}

	// task with same repo, but different branch
	id3 := "bt4brhjpc98qra498sg2"
	branch2 := "another_branch"
	err = q1.Push(&Task{
		ID: id3,
		CreatedBy: CreatedBy{
			Branch: branch2,
			Repo:   repo2,
		},
		States: states,
	})
	if err != nil {
		t.Fatal(err)
	}

	q1.RemoveExisting(branch, repo)
	assert.Equal(t, 2, q1.tq.Len())
}

func convertTask(taskData []byte) (*Task, error) {
	tsk := &Task{}
	err := json.Unmarshal(taskData, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

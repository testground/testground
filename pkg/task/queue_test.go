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

	// open queue and push an item into the queue
	q, err := NewQueue(ts, 100, convertTask)
	if err != nil {
		t.Fatal(err)
	}

	id1 := "ab4brhjpc98qra498sg0"
	branch := "test_branch"
	repo := "test_repo"
	states := []DatedState{{State: StateScheduled, Created: time.Now()}}
	err = q.Push(&Task{
		ID: id1,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo,
		},
		States:   states,
		Priority: 200,
	})

	if err != nil {
		t.Fatal(err)
	}

	// task with same branch, but different repo
	id2 := "cd4brhjpc98qra498sg1"
	repo2 := "another_repo"
	err = q.Push(&Task{
		ID: id2,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo2,
		},
		States:   states,
		Priority: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	// task with same repo, but different branch
	id3 := "cc4brhjpc98qra498sg2"
	branch2 := "another_branch"
	err = q.Push(&Task{
		ID: id3,
		CreatedBy: CreatedBy{
			Branch: branch2,
			Repo:   repo2,
		},
		States:   states,
		Priority: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	// task with same repo and same branch - should push out first task
	id4 := "hg4brhjpc98qra566sg3"
	err = q.PushUniqueByBranch(&Task{
		ID: id4,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo,
		},
		States:   states,
		Priority: 3,
	})

	if err != nil {
		t.Fatal(err)
	}

	// queue should have 3 items: the first one should be pushed out by the 4th
	assert.Equal(t, 3, q.tq.Len())

	// now pop items from queue, and check the ordering
	qt1, err := q.Pop()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, id2, qt1.ID)

	qt2, err := q.Pop()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, id3, qt2.ID)

	qt3, err := q.Pop()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, id4, qt3.ID)

	// The first task should be canceled (removed from queue)
	tsk, err := ts.Get(id1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tsk.State().State, StateCanceled)

}

func TestQueueDoesNotRemoveTasksWithoutBranchOrRepo(t *testing.T) {
	/// Both queues will use the same storage
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// open queue and push an item into the queue
	q, err := NewQueue(ts, 100, convertTask)
	if err != nil {
		t.Fatal(err)
	}

	// Push two tasks, with different IDs, and no repo/branch set
	id := "bt4brhjpc98qra498sg0"
	branch := ""
	repo := ""
	states := []DatedState{{State: StateScheduled, Created: time.Now()}}
	err = q.Push(&Task{
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

	id2 := "bt3brhjpc98qra498sg1"
	err = q.PushUniqueByBranch(&Task{
		ID: id2,
		CreatedBy: CreatedBy{
			Branch: branch,
			Repo:   repo,
		},
		States: states,
	})

	if err != nil {
		t.Fatal(err)
	}

	// Both tasks should be present in the queue
	assert.Equal(t, 2, q.tq.Len())
}

func convertTask(taskData []byte) (*Task, error) {
	tsk := &Task{}
	err := json.Unmarshal(taskData, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

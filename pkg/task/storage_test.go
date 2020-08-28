package task

import (
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"testing"
	"time"
)

func TestChangePrefix(t *testing.T) {
	id := "bt4brhjpc98qra498sg0"
	nexp := taskKey(QUEUEPREFIX, id)
	exp := taskKey(CURRENTPREFIX, id)
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// Add a task to one prefix, then change it to another.
	err = ts.Put(QUEUEPREFIX, &Task{
		ID: id,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ts.ChangePrefix(CURRENTPREFIX, QUEUEPREFIX, id)
	if err != nil {
		t.Fatal(err)
	}
	// In the database, I expect to see the task stored with the prefix
	exists, err := ts.db.Has([]byte(exp), nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, exists)
	// In the database, I expect to not see the task stored with the old prefix
	exists, err = ts.db.Has([]byte(nexp), nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, exists)
}

// While in the CURRENT task state, make sure task state entry is recorded
func TestAppendTaskState(t *testing.T) {
	id := "bt4brhjpc98qra498sg0"
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// Create a task in the current prefix so we can append states to its log.
	err = ts.Put(CURRENTPREFIX, &Task{
		ID: id,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Through the lifetime of the task running, append state events to it.
	if err := ts.AppendTaskState(id, StateProcessing); err != nil {
		t.Fatal(err)
	}
	if err := ts.AppendTaskState(id, StateComplete); err != nil {
		t.Fatal(err)
	}

	// How many states are there?
	tsk, err := ts.Get(CURRENTPREFIX, id)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(tsk.States))
}

func TestArchive(t *testing.T) {
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &Storage{db}

	// find all tasks between a certain date and time
	// I expect to find three of the tasks between this range.
	cali, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	
	// Add tasks created in different times to the archive.
	// The ID will typically be generated by the system with the clock encoded.
	// For this test, generate meaningful IDs.
	for _, id := range []string{
		"brfdnkrpc98qs6rq33b0", // 2020-06-08 17:46:11.3170344 -0700 PDT
		"brfdnnbpc98qso583v20", // 2020-06-08 17:46:21.6249457 -0700 PDT
		"brfdnq3pc98qso583v2g", // 2020-06-08 17:46:32.0048886 -0700 PDT
		"brfdnsjpc98qso583v30", // 2020-06-08 17:46:42.3196851 -0700 PDT
		"brfdnv3pc98qv5avsk90", // 2020-06-08 17:46:52.6554658 -0700 PDT
		"brfdo1rpc98r0s6e2dv0", // 2020-06-08 17:47:03.0812714 -0700 PDT
	} {
		tsk := Task{
			ID: id,
		}
		err := ts.Put(ARCHIVEPREFIX, &tsk)
		if err != nil {
			t.Fatal(err)
		}
	}

	before := time.Date(2020, 6, 8, 17, 46, 20, 0, cali)
	after := time.Date(2020, 6, 8, 17, 46, 50, 0, cali)

	between, err := ts.Range(ARCHIVEPREFIX, before, after)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, len(between))
}

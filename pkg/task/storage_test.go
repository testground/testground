package task

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestChangePrefix(t *testing.T) {
	id := "testing123"
	exp := "current:testing123"
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &TaskStorage{db}

	// Add a task to one prefix, then change it to another.
	ts.Put(QUEUEPREFIX, &Task{
		ID: id,
	})
	ts.ChangePrefix(CURRENTPREFIX, QUEUEPREFIX, id)
	// In the database, I expect to see the task stored with the prefix
	exists, err := ts.db.Has([]byte(exp), nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, exists)
}

// While in the CURRENT task state, make sure task state entry is recorded
func AppendTaskState(t *testing.T) {
	id := "testtesttest"
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := &TaskStorage{db}

	// Create a task in the current prefix so we can append states to its log.
	ts.Put(CURRENTPREFIX, &Task{
		ID: id,
	})

	// Through the lifetime of the task running, append state events to it.
	ts.AppendTaskState(id, StateProcessing)
	ts.AppendTaskState(id, StateComplete)

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
	ts := &TaskStorage{db}

	// Add tasks created in different times to the archive.
	for i, year := range []int{2000, 2005, 2010, 2015, 2020} {
		id := strconv.Itoa(i)
		created := time.Date(year, 1, 1, 1, 1, 1, 1, time.FixedZone("UTC", 0))
		tsk := Task{
			ID:      id,
			Created: created,
		}
		ts.Put(ARCHIVEPREFIX, &tsk)
	}
	// Add a few tests that have occurred at different times.

	before := time.Date(2009, 1, 1, 1, 1, 1, 1, time.FixedZone("UTC", 0))
	after := time.Date(2019, 1, 1, 1, 1, 1, 1, time.FixedZone("UTC", 0))

	between, err := ts.ArchiveRange(before, after)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(between))
}

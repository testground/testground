package task

import (
	"encoding/json"
	"errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	// database key prefixes
	QUEUEPREFIX   = "queue"
	CURRENTPREFIX = "current"
	ARCHIVEPREFIX = "archive"

	ErrNotFound = errors.New("task not found")
)

// Tasks stored in leveldb
type Storage struct {
	db *leveldb.DB
}

// derive the key from the database prefix and the ID of the task we are searching for.
// In order to do time-based range searches and searches for tasks in a particular phase of execution,
// keys are stored under a prefix which represents the state of the task and a timestamp.
// Tasks are using a time-based UUID to identify each task. Get the time from the ID.
// This works because leveldb stores keys in lexicographical order, and by doing this we make sure the time
// order and lexicographical order of the database are the same.
//
// For example, two IDs generated a few minutes apart:
// 8e1ae8c9-aa82-11ea-9feb-ccb0daba35bf  <- Timestamp: 1591728829482004100
// a26e8628-aa82-11ea-8873-ccb0daba35bf  <- Timestamp: 1591728863584413600
// By sorting the keys lexicographically by the converted timestamp, we can range query over a given period
// So key `archive:1591728829482004100` will contain the task with ID 8e1ae8c9-aa82-11ea-9feb-ccb0daba35bf
func taskKey(prefix string, id string) []byte {
	var tskey string
	u, err := uuid.Parse(id)
	if err != nil { // can't parse the ID, use the ID directly
		tskey = id
	} else { // Convert the ID into a timestamp.
		sec, usec := u.Time().UnixTime()
		tskey = strconv.FormatInt(sec, 10) + strconv.FormatInt(usec, 10)
	}
	return []byte(strings.Join([]string{prefix, tskey}, ":"))
}

func (s *Storage) Get(prefix string, id string) (tsk *Task, err error) {
	tsk = new(Task)
	val, err := s.db.Get(taskKey(prefix, id), nil)
	if err == leveldb.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(val, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

func (s *Storage) Put(prefix string, tsk *Task) error {
	val, err := json.Marshal(tsk)
	if err != nil {
		return err
	}
	return s.db.Put(taskKey(prefix, tsk.ID), val, &opt.WriteOptions{
		Sync: true,
	})
}

func (s *Storage) Delete(prefix string, tsk *Task) error {
	key := []byte(strings.Join([]string{prefix, tsk.ID}, ":"))
	return s.db.Delete(key, &opt.WriteOptions{
		Sync: true,
	})
}

// A helper method for appending a state to a task's state slice
func (s *Storage) AppendTaskState(id string, state State) error {
	tsk, err := s.Get(CURRENTPREFIX, id)
	if err != nil {
		return err
	}
	dated := DatedState{
		State:   state,
		Created: time.Now(),
	}
	tsk.States = append(tsk.States, dated)
	return s.Put(CURRENTPREFIX, tsk)
}

func (s *Storage) MarkCompleted(id string, error error, data interface{}) error {
	tsk, err := s.Get(CURRENTPREFIX, id)
	if err != nil {
		return err
	}
	dated := DatedState{
		State:   StateComplete,
		Created: time.Now(),
	}
	tsk.States = append(tsk.States, dated)
	tsk.Result = Result{
		Data: data,
	}
	if error != nil {
		tsk.Result.Error = error.Error()
	}
	err = s.Put(CURRENTPREFIX, tsk)
	if err != nil {
		return err
	}
	return s.ChangePrefix(ARCHIVEPREFIX, CURRENTPREFIX, id)
}

// Change the prefix of a task
func (s *Storage) ChangePrefix(dst string, src string, id string) error {
	oldkey := taskKey(src, id)
	newkey := taskKey(dst, id)
	trans, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}
	val, err := trans.Get(oldkey, nil)
	if err != nil {
		trans.Discard()
		return err
	}
	err = trans.Put(newkey, val, &opt.WriteOptions{Sync: true})
	if err != nil {
		trans.Discard()
		return err
	}
	err = trans.Delete(oldkey, &opt.WriteOptions{Sync: true})
	if err != nil {
		trans.Discard()
		return err
	}
	return trans.Commit()
}

// Range returns []*Task with all tasks between the given time ranges.
func (s *Storage) Range(prefix string, start time.Time, end time.Time) (tasks []*Task, err error) {
	rng := util.Range{
		Start: []byte(strings.Join([]string{
			prefix,
			strconv.FormatInt(start.Unix(), 10)}, ":")),
		Limit: []byte(strings.Join([]string{
			prefix,
			strconv.FormatInt(end.Unix(), 10)}, ":")),
	}

	tasks = make([]*Task, 0)

	iter := s.db.NewIterator(&rng, nil)
	defer iter.Release()

	for iter.Next() {
		tsk := new(Task)
		err := json.Unmarshal(iter.Value(), tsk)
		if err != nil {
			return tasks, err
		}
		tasks = append(tasks, tsk)
	}
	return tasks, nil
}

func NewMemoryTaskStorage() (*Storage, error) {
	inmem := storage.NewMemStorage()
	db, err := leveldb.Open(inmem, nil)
	if err != nil {
		return nil, err
	}
	return &Storage{db}, nil
}

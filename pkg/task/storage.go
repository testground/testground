package task

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	// database key prefixes
	QUEUEPREFIX   = "queued"
	CURRENTPREFIX = "current"
	ARCHIVEPREFIX = "archive"
)

// Tasks stored in leveldb
type TaskStorage struct {
	db *leveldb.DB
}

func (s *TaskStorage) Get(prefix string, id string) (tsk *Task, err error) {
	tsk = new(Task)
	key := []byte(strings.Join([]string{prefix, id}, ":"))
	val, err := s.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(val, tsk)
	if err != nil {
		return nil, err
	}
	return tsk, nil
}

func (s *TaskStorage) Put(prefix string, tsk *Task) error {
	key := []byte(strings.Join([]string{prefix, tsk.ID}, ":"))
	val, err := json.Marshal(tsk)
	if err != nil {
		return err
	}
	return s.db.Put(key, val, &opt.WriteOptions{
		Sync: true,
	})
}

func (s *TaskStorage) Delete(prefix string, tsk *Task) error {
	key := []byte(strings.Join([]string{prefix, tsk.ID}, ":"))
	return s.db.Delete(key, &opt.WriteOptions{
		Sync: true,
	})
}

// A helper method for appending a state to a task's state slice
func (s *TaskStorage) AppendTaskState(id string, state TaskState) error {
	tsk, err := s.Get(CURRENTPREFIX, id)
	if err != nil {
		return err
	}
	dated := DatedTaskState{
		TaskState: state,
		Entered:   time.Now(),
	}
	tsk.States = append(tsk.States, dated)
	return s.Put(CURRENTPREFIX, tsk)
}

// Change the prefix of a task
func (s *TaskStorage) ChangePrefix(dst string, src string, id string) error {
	tsk, err := s.Get(src, id)
	if err != nil {
		return err
	}
	return s.Put(dst, tsk)
}

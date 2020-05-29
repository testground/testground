package daemon

import (
	"fmt"

	"github.com/beeker1121/goque"
)

func init() {
	Q, _ = goque.OpenPriorityQueue(".", goque.DESC)
}

var Q *goque.PriorityQueue

const MAX_QUEUE_LEN uint64 = 100

func EnqueueTask(t *Task) error {
	if Q.Length() >= MAX_QUEUE_LEN {
		return fmt.Errorf("slow down; too many tasks")
	}
	_, err := Q.EnqueueObject(t.Priority, t)
	return err
}

func DequeueTask() (*Task, error) {
	task := new(Task)
	item, err := Q.Dequeue()
	if err != nil {
		return nil, err
	}
	err = item.ToObject(task)
	if err != nil {
		return nil, err
	}
	return task, nil

}

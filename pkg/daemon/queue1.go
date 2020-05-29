package daemon

import (
	"context"
	"errors"
	"time"

	"github.com/beeker1121/goque"
)

// TBD.
type Task struct {
	Priority uint8
	Who      string
	What     string
	Where    string
}

func PersistentTaskQueueWorker(ctx context.Context, data_dir string, enqueue chan *Task, dequeue chan *Task) error {
	q, err := goque.OpenPriorityQueue(data_dir, goque.DESC)
	if err != nil {
		return err
	}
	defer q.Close()

	done := make(chan bool)
	errs := make(chan error)

	go enqueueTasks(q, enqueue, errs, done)
	go dequeueTasks(q, dequeue, errs)

	for {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			close(enqueue)
			return ctx.Err()
		case err := <-errs:
			close(enqueue)
			return err
		}
	}
}

func enqueueTasks(q *goque.PriorityQueue, enqueue chan *Task, errs chan error, done chan bool) {
	for t := range enqueue {
		priority := t.Priority
		_, err := q.EnqueueObject(priority, t)
		if err != nil {
			errs <- err
			return
		}
	}
	close(done)
}

func dequeueTasks(q *goque.PriorityQueue, dequeue chan *Task, errs chan error) {
	tick := time.Tick(time.Second)
	for {
		select {
		case <-tick:
			item, err := q.Dequeue()
			if err != nil {
				if errors.Is(err, goque.ErrEmpty) {
					continue
				}
				errs <- err
			}
			task := new(Task)
			item.ToObject(task)
			dequeue <- task
		}
	}
}

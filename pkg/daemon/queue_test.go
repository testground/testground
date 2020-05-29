package daemon_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/testground/testground/pkg/daemon"
)

func TestWorkerPriority(t *testing.T) {
	enqueue := make(chan *daemon.Task)
	dequeue := make(chan *daemon.Task)
	data_dir, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(data_dir) })

	ctx := context.Background()
	go daemon.PersistentTaskQueueWorker(ctx, data_dir, enqueue, dequeue)
	for i := 0; i < 10; i++ {
		task := daemon.Task{
			Priority: uint8(i),
		}
		enqueue <- &task
	}

	next := <-dequeue
	assert.Equal(t, uint8(9), next.Priority, "queue did not sort by priority")
}

func TestNonWorkerPriority(t *testing.T) {
	for i := 0; i < 10; i++ {
		task := daemon.Task{
			Priority: uint8(i),
		}
		daemon.EnqueueTask(&task)
	}
	next, err := daemon.DequeueTask()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint8(9), next.Priority, "queue did not sort by priority")
}

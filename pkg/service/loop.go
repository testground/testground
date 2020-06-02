package service

import (
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/task"
)

func Runloop(engine api.Engine) error {
	stor := engine.TaskStorage()
	// While there is nothing in the queue, just wait
	for {
		for stor.Len() == 0 {
			logging.S().Info("nothing yet")
			time.Sleep(10 * time.Second)

		}

		logging.S().Info("popping")

		// Process task.
		// One at a time, the task is removed from the queue and executed.
		// The state of the run is updated for the benefit of the client
		tsk, err := stor.Pop()
		if err != nil {
			return err
		}

		logging.S().Info("processing ", tsk.ID)

		// TODO Actually do the build and run
		// The data is already saved in $TESTGROUND_HOME.
		// Maybe we want this to be configurable?
		stor.SetState(tsk, task.TaskStateBuilding)
		logging.S().Info("building ", tsk.ID)
		time.Sleep(5 * time.Second)
		stor.SetState(tsk, task.TaskStateRunning)
		logging.S().Info("running ", tsk.ID)
		time.Sleep(5 * time.Second)
		stor.SetState(tsk, task.TaskStateComplete)
		logging.S().Info("marking complete ", tsk.ID)

	}
}

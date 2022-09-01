package engine

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/task"
)

func TestUnmarshalTaskRun(t *testing.T) {
	taskData := &task.Task{
		Type:        task.TypeRun,
		Version:     0,
		Priority:    0,
		Plan:        "test-task-plan",
		Case:        "test-task-case",
		ID:          "test-case-id-123456",
		Runner:      "exec:go",
		Composition: nil,
		Input: &RunInput{
			RunRequest: &api.RunRequest{
				CreatedBy:   api.CreatedBy{User: "test-user"},
				BuildGroups: []int{1, 2, 3},
			},
			Sources: &api.UnpackedSources{BaseDir: "home/dir/", PlanDir: "home/plan/dir"},
		},
		States: []task.DatedState{
			{
				State:   task.StateScheduled,
				Created: time.Now().UTC(),
			},
		},
		CreatedBy: task.CreatedBy(api.CreatedBy{User: "test-user"}),
	}

	jsonData, err := json.Marshal(taskData)

	if err != nil {
		t.Fatalf("error marshaling task to json: %s", err)
	}

	newTask, err := UnmarshalTask(jsonData)

	if err != nil {
		t.Fatalf("error unmarshaling task from json: %s", err)
	}

	if !reflect.DeepEqual(newTask, taskData) {
		t.Errorf("Unmarshal Run task returned incorrect data")
	}
}

func TestUnmarshalTaskBuild(t *testing.T) {
	taskData := &task.Task{
		Type:        task.TypeBuild,
		Version:     0,
		Priority:    0,
		Plan:        "test-task-plan",
		Case:        "test-task-case",
		ID:          "test-case-id-123456",
		Runner:      "exec:go",
		Composition: nil,
		Input: &BuildInput{
			BuildRequest: &api.BuildRequest{
				CreatedBy: api.CreatedBy{User: "test-user"},
				Composition: api.Composition{
					Groups: api.Groups{&api.Group{ID: "1234"}}},
			},
			Sources: &api.UnpackedSources{BaseDir: "home/dir/", PlanDir: "home/plan/dir"},
		},
		States: []task.DatedState{
			{
				State:   task.StateScheduled,
				Created: time.Now().UTC(),
			},
		},
		CreatedBy: task.CreatedBy(api.CreatedBy{User: "test-user"}),
	}

	jsonData, err := json.Marshal(taskData)

	if err != nil {
		t.Fatalf("error marshaling task to json: %s", err)
	}

	newTask, err := UnmarshalTask(jsonData)

	if err != nil {
		t.Fatalf("error unmarshaling task from json: %s", err)
	}

	if !reflect.DeepEqual(newTask, taskData) {
		t.Errorf("Unmarshal Build task returned incorrect data")
	}
}

func TestEngineCleanupTask(t *testing.T) {
	daemonConfig := config.DaemonConfig{Scheduler: config.SchedulerConfig{TaskRepoType: "memory"}}
	envConfig := &config.EnvConfig{Daemon: daemonConfig}
	engineConfig := EngineConfig{EnvConfig: envConfig}
	e, err := NewEngine(&engineConfig)
	if err != nil {
		t.Error(err)
	}

	inmem := e.store

	// Add tasks to storage
	// First task: in processing (but no signal connected)
	id1 := "bt4brhjpc98qra498sg0"
	tsk1 := &task.Task{ID: id1, Type: task.TypeBuild}
	tsk1.States = append(tsk1.States, task.DatedState{Created: time.Now(), State: task.StateProcessing})
	err = inmem.PersistProcessing(tsk1)
	if err != nil {
		t.Error(err)
	}

	// Second: scheduled
	id2 := "bt4brhjpc98qra498sg1"
	tsk2 := &task.Task{ID: id2, Type: task.TypeBuild}
	tsk2.States = append(tsk2.States, task.DatedState{Created: time.Now(), State: task.StateScheduled})
	err = inmem.PersistScheduled(tsk2)
	if err != nil {
		t.Error(err)
	}

	// Third: in processing, with a connected signal
	id3 := "bt4brhjpc98qra498sg2"
	tsk3 := &task.Task{ID: id3, Type: task.TypeBuild}
	tsk3.States = append(tsk3.States, task.DatedState{Created: time.Now(), State: task.StateProcessing})
	err = inmem.PersistProcessing(tsk3)
	if err != nil {
		t.Error(err)
	}

	task3ch := make(chan int)
	e.addSignal(id3, task3ch)

	err = e.CleanUpTasks()
	if err != nil {
		t.Error(err)
	}

	tasks, err := e.store.AllTasks()
	if err != nil {
		t.Error(err)
	}

	// 2 tasks should remain
	assert.Equal(t, 2, len(tasks))

	// the scheduled, and the one in active processing
	assert.Equal(t, id2, tasks[1].ID)
	assert.Equal(t, id3, tasks[0].ID)
}

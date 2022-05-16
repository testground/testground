package engine

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/testground/testground/pkg/api"
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

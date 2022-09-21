package engine

import (
	"encoding/json"
	"github.com/otiai10/copy"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/config"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/task"
)

type Testcase string

const (
	Ok    Testcase = "ok"
	Stall Testcase = "stall"
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

func TestNewEngine(t *testing.T) {
	env := &config.EnvConfig{
		Daemon: config.DaemonConfig{
			Scheduler: config.SchedulerConfig{
				TaskRepoType:   "memory",
				Workers:        1,
				TaskTimeoutMin: 1,
			},
		},
		Runners: map[string]config.ConfigMap{
			"local:exec": {},
		},
		Builders: map[string]config.ConfigMap{
			"exec:go": {},
		},
	}
	err := env.Load()
	require.NoError(t, err)

	cfg := &EngineConfig{
		Builders:  AllBuilders,
		Runners:   AllRunners,
		EnvConfig: env,
	}

	engine, err := NewEngine(cfg)
	require.NoError(t, err)

	t.Run("OK case", func(t *testing.T) {
		testTask := createTestTask(t, Ok)
		require.NoError(t, engine.queue.Push(testTask))
		time.Sleep(30 * time.Second) // need to give it a time to finish because it is asynchronous function.
		// get the task back from the database and check final state
		taskFromDB, err := engine.store.Get(testTask.ID)
		require.NoError(t, err)
		require.Equal(t, task.StateComplete, taskFromDB.State().State)
	})

	t.Run("Stall case", func(t *testing.T) {
		testTask := createTestTask(t, Stall)
		require.NoError(t, engine.queue.Push(testTask))
		time.Sleep(70 * time.Second) // need to give it a time to finish because it is asynchronous function.
		// get the task back from the database and check final state
		taskFromDB, err := engine.store.Get(testTask.ID)
		require.NoError(t, err)
		require.Equal(t, task.StateCanceled, taskFromDB.State().State)
	})
}

// testcase could be one of these: 'ok' and 'stall'
// OK will execute right away and stall will cause deadline exceed.
func createTestTask(t *testing.T, testcase Testcase) *task.Task {
	cmp := api.Composition{
		Global: api.Global{
			Builder:        "exec:go",
			Plan:           "placebo",
			Case:           string(testcase),
			TotalInstances: 1,
			BuildConfig: map[string]interface{}{
				"go_proxy_mode": "direct",
			},
			Runner: "local:exec",
			RunConfig: map[string]interface{}{
				"enabled": true,
			},
		},
		Groups: []*api.Group{
			{
				ID:        "test",
				Instances: api.Instances{Count: 1},
			},
		},
	}

	manifest := api.TestPlanManifest{
		Name: "placebo",
		Builders: map[string]config.ConfigMap{
			"exec:go": {},
		},
		Runners: map[string]config.ConfigMap{
			"local:exec": {},
		},
		TestCases: []*api.TestCase{
			{
				Name:      string(testcase),
				Instances: api.InstanceConstraints{Minimum: 1, Maximum: 100},
				Parameters: map[string]api.Parameter{
					"param4": {
						Type:    "string",
						Default: "value4:default:manifest",
					},
				},
			},
		},
	}
	basedir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil
	}

	plandir := filepath.Join(basedir, "plan")
	err = copy.Copy("../../plans/placebo", plandir)
	if err != nil {
		return nil
	}
	t.Cleanup(func() {
		os.RemoveAll(basedir)
	})

	id := xid.New().String()
	taskData := &task.Task{
		Type:        task.TypeRun,
		Version:     0,
		Priority:    0,
		Plan:        "placebo",
		Case:        string(testcase),
		ID:          id,
		Runner:      "local:exec",
		Composition: cmp,
		Input: RunInput{
			RunRequest: &api.RunRequest{
				CreatedBy:   api.CreatedBy{User: "test-user"},
				BuildGroups: []int{0},
				Composition: cmp,
				Manifest:    manifest,
			},

			Sources: &api.UnpackedSources{BaseDir: basedir, PlanDir: plandir},
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
		return nil
	}

	newTask, err := UnmarshalTask(jsonData)

	if err != nil {
		return nil
	}

	return newTask
}

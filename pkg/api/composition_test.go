package api

import (
	"encoding/json"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/task"
)

func TestValidateGroupsUnique(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:    "foo_plan",
			Case:    "foo_case",
			Builder: "docker:go",
			Runner:  "local:docker",
		},
		Groups: []*Group{
			{ID: "repeated"},
			{ID: "repeated"},
		},
	}

	require.Error(t, c.ValidateForBuild())
	require.Error(t, c.ValidateForRun())
}

func TestValidateGroupBuildKey(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:    "foo_plan",
			Case:    "foo_case",
			Builder: "docker:go",
			Runner:  "local:docker",
		},
		Groups: []*Group{
			{
				ID:      "repeated",
				Builder: "docker:go",
			},
			{
				ID:      "another-id",
				Builder: "docker:go",
			},
			{
				ID:      "custom-selector",
				Builder: "docker:go",
				Build: Build{
					Selectors: []string{"a", "b"},
				},
			},
			{
				ID:      "duplicate-selector",
				Builder: "docker:go",
				Build: Build{
					Selectors: []string{"a", "b"},
				},
			},
			{
				ID:      "duplicate-selector-with-different-build-config",
				Builder: "docker:go",
				Build: Build{
					Selectors: []string{"a", "b"},
				},
				BuildConfig: map[string]interface{}{
					"dockerfile_extensions": map[string]string{
						"pre_mod_download": "pre_mod_download_overriden",
					},
				},
			},
		},
	}

	k0 := c.Groups[0].BuildKey() // repeated
	k1 := c.Groups[1].BuildKey() // another-id

	k2 := c.Groups[2].BuildKey() // custom-selector
	k3 := c.Groups[3].BuildKey() // duplicate-selector

	k4 := c.Groups[4].BuildKey() // duplicate-selector-with-different-build-config

	require.EqualValues(t, k0, k1)
	require.EqualValues(t, k2, k3)
	require.NotEqualValues(t, k0, k2)

	require.NotEqualValues(t, k3, k4)
}

func TestTotalInstancesIsComputedWhenPossible(t *testing.T) {
	// when all groups have a fixed number of instances, the total is computed
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:    "foo_plan",
			Case:    "foo_case",
			Builder: "docker:go",
			Runner:  "local:docker",
		},
		Groups: []*Group{
			{
				ID:        "a",
				Builder:   "docker:generic",
				Instances: Instances{Count: 3},
			},
			{
				ID:        "b",
				Instances: Instances{Count: 2},
			},
			{
				ID:        "c",
				Instances: Instances{Count: 1},
			},
		},
	}
	c = c.GenerateDefaultRun()

	err := c.ValidateForBuild()
	require.NoError(t, err)

	err = c.ValidateForRun()
	require.NoError(t, err)

	require.EqualValues(t, 6, c.Runs[0].TotalInstances)

	// when some groups have a percentage, the total can't be computed
	c = &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:    "foo_plan",
			Case:    "foo_case",
			Builder: "docker:go",
			Runner:  "local:docker",
		},
		Groups: []*Group{
			{
				ID:        "a",
				Builder:   "docker:generic",
				Instances: Instances{Count: 3},
			},
			{
				ID:        "b",
				Instances: Instances{Percentage: 50},
			},
		},
	}
	c = c.GenerateDefaultRun()

	err = c.ValidateForBuild()
	require.NoError(t, err)

	err = c.ValidateForRun()
	require.Error(t, err)

	require.EqualValues(t, 0, c.Runs[0].TotalInstances)

	// when groups mix percentages and fixed numbers, the total is verified
	c = &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			Builder:        "docker:go",
			Runner:         "local:docker",
			TotalInstances: 6,
		},
		Groups: []*Group{
			{
				ID:        "a",
				Builder:   "docker:generic",
				Instances: Instances{Count: 3},
			},
			{
				ID:        "b",
				Instances: Instances{Percentage: 0.5},
			},
		},
	}
	c = c.GenerateDefaultRun()

	err = c.ValidateForBuild()
	require.NoError(t, err)

	err = c.ValidateForRun()
	require.NoError(t, err)

	require.EqualValues(t, 6, c.Runs[0].TotalInstances)

	// when groups uses percentages that don't work with the total, the total is verified
	c = &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			Builder:        "docker:go",
			Runner:         "local:docker",
			TotalInstances: 6,
		},
		Groups: []*Group{
			{
				ID:        "a",
				Builder:   "docker:generic",
				Instances: Instances{Count: 3},
			},
			{
				ID:        "b",
				Instances: Instances{Percentage: 1.2},
			},
		},
	}
	c = c.GenerateDefaultRun()

	err = c.ValidateForBuild()
	require.NoError(t, err)

	err = c.ValidateForRun()
	require.Error(t, err)
}

func TestListBuilders(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:    "foo_plan",
			Case:    "foo_case",
			Builder: "docker:go",
			Runner:  "local:docker",
		},
		Groups: []*Group{
			{
				ID:      "repeated",
				Builder: "docker:generic",
			},
			{
				ID: "another-id",
			},
		},
	}

	require.EqualValues(t, []string{"docker:generic", "docker:go"}, c.ListBuilders())
}

func TestBuildKeyWithoutBuilderPanics(t *testing.T) {
	defer func() { _ = recover() }()

	g := &Group{
		ID: "no-info-should-throw",
	}

	g.BuildKey()
	t.Errorf("did not panic")
}

func TestBuildKeyDependsOnBuilder(t *testing.T) {
	g1 := &Group{
		ID:      "with-generic",
		Builder: "docker:generic",
	}

	g2 := &Group{
		ID:      "with-go",
		Builder: "docker:go",
	}

	g3 := &Group{
		ID:      "another-with-go",
		Builder: "docker:go",
	}

	k1 := g1.BuildKey()
	k2 := g2.BuildKey()
	k3 := g3.BuildKey()

	require.NotEqualValues(t, k1, k2)
	require.EqualValues(t, k2, k3)
}

func TestGroupsMayDefineBuilder(t *testing.T) {
	g := &Group{
		ID:      "foo",
		Builder: "docker:generic",
	}

	require.NotNil(t, g)
}

func TestIssue1493CompositionContainsARunsField(t *testing.T) {
	// Composition with global builder and group builder.
	globalWithBuilder := Global{
		Plan:           "foo_plan",
		Case:           "foo_case",
		Builder:        "docker:go",
		Runner:         "local:docker",
		TotalInstances: 3,
	}

	group := &Group{
		ID: "foo",
	}

	run := &Run{
		ID: "foo",
	}

	validComposition := &Composition{
		Metadata: Metadata{},
		Global:   globalWithBuilder,
		Groups:   []*Group{group},
		Runs:     []*Run{run},
	}

	require.NotNil(t, validComposition.Runs)
}

func TestListRunAndGroupsIds(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 4,
			Builder:        "docker:go",
			Runner:         "local:docker",
			BuildConfig: map[string]interface{}{
				"build_base_image": "base_image_global",
			},
		},
		Groups: []*Group{
			{
				ID: "a",
			},
			{
				ID: "b",
			},
			{
				ID: "c",
			},
			{
				ID:      "d",
				Builder: "docker:generic",
			},
		},
		Runs: []*Run{
			{
				ID: "aa",
			},
			{
				ID: "bb",
			},
			{
				ID: "cc",
			},
		},
	}

	groups := c.ListGroupsIds()
	require.EqualValues(t, []string{"a", "b", "c", "d"}, groups)

	runs := c.ListRunIds()
	require.EqualValues(t, []string{"aa", "bb", "cc"}, runs)

}

func TestFrameForRun(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 4,
			Builder:        "docker:go",
			Runner:         "local:docker",
			BuildConfig: map[string]interface{}{
				"build_base_image": "base_image_global",
			},
		},
		Groups: []*Group{
			{
				ID: "a",
			},
			{
				ID: "b",
			},
			{
				ID: "c",
			},
			{
				ID:      "d",
				Builder: "docker:generic",
			},
		},
		Runs: []*Run{
			{
				ID: "just-a",
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 3},
					},
				},
			},
			{
				ID: "a-and-b",
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 3},
					},
					{
						ID:        "b",
						Instances: Instances{Count: 1},
					},
				},
			},
			{
				ID: "a-and-c",
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 3},
					},
					{
						ID:        "c",
						Instances: Instances{Count: 4},
					},
				},
			},
		},
	}

	framedForRunA, err := c.FrameForRuns("just-a")
	require.NoError(t, err)

	groupIds := framedForRunA.ListGroupsIds()
	runIds := framedForRunA.ListRunIds()

	// require.EqualValues(t, 3, framedForRunA.Global.TotalInstances) TODO
	require.EqualValues(t, []string{"a"}, groupIds)
	require.EqualValues(t, []string{"just-a"}, runIds)

	framedForRunAAndB, err := c.FrameForRuns("a-and-b")
	require.NoError(t, err)

	groupIds = framedForRunAAndB.ListGroupsIds()
	runIds = framedForRunAAndB.ListRunIds()

	// require.EqualValues(t, 4, framedForRunAAndB.Global.TotalInstances) TODO
	require.EqualValues(t, []string{"a", "b"}, groupIds)
	require.EqualValues(t, []string{"a-and-b"}, runIds)

	framedForRunAAndC, err := c.FrameForRuns("a-and-c", "just-a")
	require.NoError(t, err)

	groupIds = framedForRunAAndC.ListGroupsIds()
	runIds = framedForRunAAndC.ListRunIds()

	// require.EqualValues(t, 10, framedForRunAAndC.Global.TotalInstances) TODO
	require.EqualValues(t, []string{"a", "c"}, groupIds)
	require.EqualValues(t, []string{"a-and-c", "just-a"}, runIds)
}

func TestGetRun(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 4,
			Builder:        "docker:go",
			Runner:         "local:docker",
			BuildConfig: map[string]interface{}{
				"build_base_image": "base_image_global",
			},
		},
		Groups: []*Group{
			{
				ID: "a",
			},
			{
				ID: "b",
			},
			{
				ID: "c",
			},
			{
				ID:      "d",
				Builder: "docker:generic",
			},
		},
		Runs: []*Run{
			{
				ID: "a",
			},
			{
				ID: "b",
			},
			{
				ID: "c",
			},
		},
	}

	run, err := c.getRun("a")

	require.NoError(t, err)
	require.EqualValues(t, "a", run.ID)

	run, err = c.getRun("d")

	require.Error(t, err)
	require.Nil(t, run)
}

func TestMarshalIsIdempotent(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 28,
			Builder:        "docker:go",
			Runner:         "local:docker",
			BuildConfig: map[string]interface{}{
				"build_base_image": "base_image_global",
			},
		},
		Groups: []*Group{
			{
				ID: "a",
				BuildConfig: map[string]interface{}{
					"build_base_image": "custom_image",
				},
			},
			{
				ID:      "b",
				Builder: "docker:go",
			},
		},
		Runs: []*Run{
			{
				ID: "just-a",
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 3},
					},
				},
			},
			{
				ID:             "a-and-b",
				TotalInstances: 4,
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 3},
					},
					{
						ID:        "b",
						Instances: Instances{Count: 1},
					},
				},
			},
		},
	}

	tsk := &task.Task{
		Composition: c,
	}

	// Marshal the task
	b, err := json.Marshal(tsk)
	require.NoError(t, err)

	// Unmarshall the task
	tsk2 := &task.Task{}
	err = json.Unmarshal(b, tsk2)
	require.NoError(t, err)

	// Decode the task
	var composition Composition
	err = mapstructure.Decode(tsk2.Composition, &composition)
	require.NoError(t, err)

	// Check equalities
	require.Equal(t, c, &composition)
	require.Equal(t, uint(4), composition.Runs[1].TotalInstances)
}

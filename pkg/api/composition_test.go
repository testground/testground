package api

import (
	"testing"

	"github.com/testground/testground/pkg/config"

	"github.com/stretchr/testify/require"
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

func TestDefaultTestParamsApplied(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 3,
			Builder:        "docker:go",
			Runner:         "local:docker",
			Run: &Run{
				TestParams: map[string]string{
					"param1": "value1:default:composition",
					"param2": "value2:default:composition",
					"param3": "value3:default:composition",
				},
			},
		},
		Groups: []*Group{
			{
				ID:        "all_set",
				Instances: Instances{Count: 1},
				Run: Run{
					TestParams: map[string]string{
						"param1": "value1:set",
						"param2": "value2:set",
						"param3": "value3:set",
					},
				},
			},
			{
				ID:        "none_set",
				Instances: Instances{Count: 1},
			},
			{
				ID:        "first_set",
				Instances: Instances{Count: 1},
				Run: Run{
					TestParams: map[string]string{
						"param1": "value1:set",
					},
				},
			},
		},
	}

	manifest := &TestPlanManifest{
		Name: "foo_plan",
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
		Runners: map[string]config.ConfigMap{
			"local:docker": {},
		},
		TestCases: []*TestCase{
			{
				Name:      "foo_case",
				Instances: InstanceConstraints{Minimum: 1, Maximum: 100},
				Parameters: map[string]Parameter{
					"param4": {
						Type:    "string",
						Default: "value4:default:manifest",
					},
				},
			},
		},
	}

	ret, err := c.PrepareForRun(manifest)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// group all_set.
	require.EqualValues(t, "value1:set", ret.Groups[0].Run.TestParams["param1"])
	require.EqualValues(t, "value2:set", ret.Groups[0].Run.TestParams["param2"])
	require.EqualValues(t, "value3:set", ret.Groups[0].Run.TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Groups[0].Run.TestParams["param4"])

	// group none_set.
	require.EqualValues(t, "value1:default:composition", ret.Groups[1].Run.TestParams["param1"])
	require.EqualValues(t, "value2:default:composition", ret.Groups[1].Run.TestParams["param2"])
	require.EqualValues(t, "value3:default:composition", ret.Groups[1].Run.TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Groups[1].Run.TestParams["param4"])

	// group first_set
	require.EqualValues(t, "value1:set", ret.Groups[2].Run.TestParams["param1"])
	require.EqualValues(t, "value2:default:composition", ret.Groups[2].Run.TestParams["param2"])
	require.EqualValues(t, "value3:default:composition", ret.Groups[2].Run.TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Groups[2].Run.TestParams["param4"])
}

func TestDefaultBuildParamsApplied(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 3,
			Builder:        "docker:go",
			Runner:         "local:docker",
			Build: &Build{
				Selectors: []string{"default_selector_1", "default_selector_2"},
				Dependencies: []Dependency{
					{"dependency:a", "1.0.0.default"},
					{"dependency:b", "2.0.0.default"},
				},
			},
		},
		Groups: []*Group{
			{
				ID: "no_local_settings",
			},
			{
				ID: "dep_override",
				Build: Build{
					Dependencies: []Dependency{
						{"dependency:a", "1.0.0.overridden"},
						{"dependency:c", "1.0.0.locally_set"},
					},
				},
			},
			{
				ID: "selector_and_dep_override",
				Build: Build{
					Selectors: []string{"overridden"},
					Dependencies: []Dependency{
						{"dependency:a", "1.0.0.overridden"},
						{"dependency:c", "1.0.0.locally_set"},
					},
				},
			},
		},
	}

	manifest := &TestPlanManifest{
		Name: "foo_plan",
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
		Runners: map[string]config.ConfigMap{
			"local:docker": {},
		},
		TestCases: []*TestCase{
			{
				Name:      "foo_case",
				Instances: InstanceConstraints{Minimum: 1, Maximum: 100},
			},
		},
	}

	ret, err := c.PrepareForBuild(manifest)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// group no_local_settings.
	require.EqualValues(t, []string{"default_selector_1", "default_selector_2"}, ret.Groups[0].Build.Selectors)
	require.ElementsMatch(t, Dependencies{{"dependency:a", "1.0.0.default"}, {"dependency:b", "2.0.0.default"}}, ret.Groups[0].Build.Dependencies)

	// group dep_override.
	require.EqualValues(t, []string{"default_selector_1", "default_selector_2"}, ret.Groups[1].Build.Selectors)
	require.ElementsMatch(t, Dependencies{
		{"dependency:a", "1.0.0.overridden"},
		{"dependency:b", "2.0.0.default"},
		{"dependency:c", "1.0.0.locally_set"},
	}, ret.Groups[1].Build.Dependencies)

	// group selector_and_dep_override
	require.EqualValues(t, []string{"overridden"}, ret.Groups[2].Build.Selectors)
	require.ElementsMatch(t, Dependencies{
		{"dependency:a", "1.0.0.overridden"},
		{"dependency:b", "2.0.0.default"},
		{"dependency:c", "1.0.0.locally_set"},
	}, ret.Groups[2].Build.Dependencies)
}

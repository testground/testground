package api

import (
	"testing"

	"github.com/testground/testground/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestDefaultTestParamsApplied(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			TotalInstances: 3,
			Builder:        "docker:go",
			Runner:         "local:docker",
			Run: &RunParams{
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
				Run: RunParams{
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
				Run: RunParams{
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
	require.EqualValues(t, "value1:set", ret.Runs[0].Groups[0].TestParams["param1"])
	require.EqualValues(t, "value2:set", ret.Runs[0].Groups[0].TestParams["param2"])
	require.EqualValues(t, "value3:set", ret.Runs[0].Groups[0].TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Runs[0].Groups[0].TestParams["param4"])

	// group none_set.
	require.EqualValues(t, "value1:default:composition", ret.Runs[0].Groups[1].TestParams["param1"])
	require.EqualValues(t, "value2:default:composition", ret.Runs[0].Groups[1].TestParams["param2"])
	require.EqualValues(t, "value3:default:composition", ret.Runs[0].Groups[1].TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Runs[0].Groups[1].TestParams["param4"])

	// group first_set
	require.EqualValues(t, "value1:set", ret.Runs[0].Groups[2].TestParams["param1"])
	require.EqualValues(t, "value2:default:composition", ret.Runs[0].Groups[2].TestParams["param2"])
	require.EqualValues(t, "value3:default:composition", ret.Runs[0].Groups[2].TestParams["param3"])
	require.EqualValues(t, "value4:default:manifest", ret.Runs[0].Groups[2].TestParams["param4"])
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
					{"dependency:a", "", "1.0.0.default"},
					{"dependency:b", "", "2.0.0.default"},
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
						{"dependency:a", "", "1.0.0.overridden"},
						{"dependency:c", "", "1.0.0.locally_set"},
						{"dependency:d", "remote/fork", "1.0.0.locally_set"},
					},
				},
			},
			{
				ID: "selector_and_dep_override",
				Build: Build{
					Selectors: []string{"overridden"},
					Dependencies: []Dependency{
						{"dependency:a", "", "1.0.0.overridden"},
						{"dependency:c", "", "1.0.0.locally_set"},
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
	require.ElementsMatch(t, Dependencies{{"dependency:a", "", "1.0.0.default"}, {"dependency:b", "", "2.0.0.default"}}, ret.Groups[0].Build.Dependencies)

	// group dep_override.
	require.EqualValues(t, []string{"default_selector_1", "default_selector_2"}, ret.Groups[1].Build.Selectors)
	require.ElementsMatch(t, Dependencies{
		{"dependency:a", "", "1.0.0.overridden"},
		{"dependency:b", "", "2.0.0.default"},
		{"dependency:c", "", "1.0.0.locally_set"},
		{"dependency:d", "remote/fork", "1.0.0.locally_set"},
	}, ret.Groups[1].Build.Dependencies)

	// group selector_and_dep_override
	require.EqualValues(t, []string{"overridden"}, ret.Groups[2].Build.Selectors)
	require.ElementsMatch(t, Dependencies{
		{"dependency:a", "", "1.0.0.overridden"},
		{"dependency:b", "", "2.0.0.default"},
		{"dependency:c", "", "1.0.0.locally_set"},
	}, ret.Groups[2].Build.Dependencies)
}

func TestDefaultBuildConfigTrickleDown(t *testing.T) {
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
				ID: "no_local_settings",
			},
			{
				ID: "dockerfile_override",
				BuildConfig: map[string]interface{}{
					"dockerfile_extensions": map[string]string{
						"pre_mod_download": "pre_mod_download_overriden",
					},
				},
			},
			{
				ID: "build_base_image_override",
				BuildConfig: map[string]interface{}{
					"build_base_image": "base_image_overriden",
				},
			},
			{
				ID:      "builder_override",
				Builder: "docker:generic",
			},
		},
	}

	manifest := &TestPlanManifest{
		Name: "foo_plan",
		Builders: map[string]config.ConfigMap{
			"docker:go": {
				"dockerfile_extensions": map[string]string{
					"pre_mod_download": "base_pre_mod_download",
				},
			},
			"docker:generic": {},
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

	// trickle down group no_local_settings.
	require.EqualValues(t, map[string]string{"pre_mod_download": "base_pre_mod_download"}, ret.Groups[0].BuildConfig["dockerfile_extensions"])
	require.EqualValues(t, "base_image_global", ret.Groups[0].BuildConfig["build_base_image"])

	// trickle down group dockerfile_override.
	require.EqualValues(t, map[string]string{"pre_mod_download": "pre_mod_download_overriden"}, ret.Groups[1].BuildConfig["dockerfile_extensions"])
	require.EqualValues(t, "base_image_global", ret.Groups[1].BuildConfig["build_base_image"])

	// trickle down group build_base_image_override.
	require.EqualValues(t, map[string]string{"pre_mod_download": "base_pre_mod_download"}, ret.Groups[2].BuildConfig["dockerfile_extensions"])
	require.EqualValues(t, "base_image_overriden", ret.Groups[2].BuildConfig["build_base_image"])

	// trickle down builder override.
	require.EqualValues(t, "docker:go", ret.Groups[0].Builder)
	require.EqualValues(t, "docker:go", ret.Groups[1].Builder)
	require.EqualValues(t, "docker:generic", ret.Groups[3].Builder)
}

func TestValidateForBuildVerifiesThatBuildersAreDefined(t *testing.T) {
	manifest := &TestPlanManifest{
		Name: "foo_plan",
		Builders: map[string]config.ConfigMap{
			"docker:go":      {},
			"docker:generic": {},
		},
	}

	// Composition with global builder and group builder.
	globalWithBuilder := Global{
		Plan:           "foo_plan",
		Case:           "foo_case",
		Builder:        "docker:go",
		Runner:         "local:docker",
		TotalInstances: 3,
	}

	groupWithoutBuilder := &Group{
		ID: "foo",
	}

	validComposition := &Composition{
		Metadata: Metadata{},
		Global:   globalWithBuilder,
		Groups:   []*Group{groupWithoutBuilder},
	}

	err := validComposition.ValidateForBuild()
	require.Nil(t, err)

	ret, err := validComposition.PrepareForBuild(manifest)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// Composition without global builder but with group builder.
	globalWithoutBuilder := Global{
		Plan:           "foo_plan",
		Case:           "foo_case",
		Runner:         "local:docker",
		TotalInstances: 3,
	}

	groupWithBuilder := &Group{
		ID:      "foo",
		Builder: "docker:generic",
	}

	validComposition2 := &Composition{
		Metadata: Metadata{},
		Global:   globalWithoutBuilder,
		Groups:   []*Group{groupWithBuilder},
	}

	err = validComposition2.ValidateForBuild()
	require.Nil(t, err)

	ret, err = validComposition2.PrepareForBuild(manifest)
	require.NoError(t, err)
	require.NotNil(t, ret)

	// Composition without global builder and without group builder.
	globalWithoutBuilder = Global{
		Plan:           "foo_plan",
		Case:           "foo_case",
		Runner:         "local:docker",
		TotalInstances: 3,
	}

	groupWithoutBuilder = &Group{
		ID: "foo",
	}

	invalidComposition := &Composition{
		Metadata: Metadata{},
		Global:   globalWithoutBuilder,
		Groups:   []*Group{groupWithoutBuilder},
	}

	err = invalidComposition.ValidateForBuild()
	require.Error(t, err)

	ret, err = invalidComposition.PrepareForBuild(manifest)
	require.Error(t, err)
	require.Nil(t, ret)
}

func TestPrepareForRunCountIsCorrect(t *testing.T) {
	manifest := &TestPlanManifest{
		Name: "foo_plan",
		TestCases: []*TestCase{
			{
				Name:      "foo_case",
				Instances: InstanceConstraints{Minimum: 1, Maximum: 100},
			},
		},
		Builders: map[string]config.ConfigMap{
			"docker:go":      {},
			"docker:generic": {},
		},
		Runners: map[string]config.ConfigMap{
			"local:docker": {},
		},
	}

	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			Plan:           "foo_plan",
			Case:           "foo_case",
			Builder:        "docker:go",
			Runner:         "local:docker",
			TotalInstances: 3,
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
		},
		Runs: []*Run{
			{
				ID: "test_a",
				Groups: []*CompositionRunGroup{
					{
						ID:        "a",
						Instances: Instances{Count: 12},
					},
				},
			},
		},
	}

	c, err := c.PrepareForRun(manifest)

	require.NotNil(t, c)
	require.NoError(t, err)
}

func TestRunConfigTrickleDown(t *testing.T) {
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
			Run: &RunParams{
				TestParams: map[string]string{
					"test_param_global": "test_param_global",
				},
			},
		},
		Groups: []*Group{
			{
				ID: "no_local_settings",
				Run: RunParams{
					TestParams: map[string]string{
						"test_param_group": "test_param_group",
					},
				},
			},
			{
				ID: "with_overrides",
				Run: RunParams{
					TestParams: map[string]string{
						"test_param_group":  "test_param_group",
						"test_param_global": "overriden_by_group",
					},
				},
			},
		},
		Runs: []*Run{
			{
				ID: "run_a",
				Groups: []*CompositionRunGroup{
					{
						ID:      "no_local_settings",
						GroupID: "no_local_settings",
						Instances: Instances{
							Count: 2,
						},
					},
					{
						ID:      "with_local_settings",
						GroupID: "with_overrides",
						Instances: Instances{
							Count: 2,
						},
						TestParams: map[string]string{
							"test_param_run": "test_param_run",
						},
					},

					{
						ID:      "with_overrides",
						GroupID: "with_overrides",
						Instances: Instances{
							Count: 2,
						},
						TestParams: map[string]string{
							"test_param_run":    "test_param_run",
							"test_param_group":  "overriden_by_run",
							"test_param_global": "overriden_by_run",
						},
					},
				},
			},
			{
				ID: "run_b",
				TestParams: map[string]string{
					"test_param_group": "override_by_runs",
					"test_param_runs":  "test_param_runs",
				},
				Groups: []*CompositionRunGroup{
					{
						ID:      "no_local_settings",
						GroupID: "no_local_settings",
						Instances: Instances{
							Count: 2,
						},
					},
					{
						ID:      "with_local_settings",
						GroupID: "with_overrides",
						Instances: Instances{
							Count: 2,
						},
						TestParams: map[string]string{
							"test_param_run": "test_param_run",
						},
					},

					{
						ID:      "with_overrides",
						GroupID: "with_overrides",
						Instances: Instances{
							Count: 2,
						},
						TestParams: map[string]string{
							"test_param_run":    "test_param_run",
							"test_param_group":  "overriden_by_run",
							"test_param_global": "overriden_by_run",
							"test_param_runs":   "overriden_by_run",
						},
					},
				},
			},
		},
	}

	manifest := &TestPlanManifest{
		Name: "foo_plan",
		Builders: map[string]config.ConfigMap{
			"docker:go": {
				"dockerfile_extensions": map[string]string{
					"pre_mod_download": "base_pre_mod_download",
				},
			},
			"docker:generic": {},
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

	ret, err := c.PrepareForRun(manifest)
	require.NoError(t, err)
	require.NotNil(t, ret)

	require.EqualValues(t, map[string]string{"test_param_global": "test_param_global", "test_param_group": "test_param_group"}, ret.Runs[0].Groups[0].TestParams)
	require.EqualValues(t, map[string]string{"test_param_global": "overriden_by_group", "test_param_group": "test_param_group", "test_param_run": "test_param_run"}, ret.Runs[0].Groups[1].TestParams)
	require.EqualValues(t, map[string]string{"test_param_global": "overriden_by_run", "test_param_group": "overriden_by_run", "test_param_run": "test_param_run"}, ret.Runs[0].Groups[2].TestParams)

	require.EqualValues(t, map[string]string{"test_param_global": "test_param_global", "test_param_group": "override_by_runs", "test_param_runs": "test_param_runs"}, ret.Runs[1].Groups[0].TestParams)
	require.EqualValues(t, map[string]string{"test_param_global": "overriden_by_group", "test_param_group": "override_by_runs", "test_param_runs": "test_param_runs", "test_param_run": "test_param_run"}, ret.Runs[1].Groups[1].TestParams)
	require.EqualValues(t, map[string]string{"test_param_global": "overriden_by_run", "test_param_group": "overriden_by_run", "test_param_runs": "overriden_by_run", "test_param_run": "test_param_run"}, ret.Runs[1].Groups[2].TestParams)

}

package api

import (
	"testing"

	"github.com/testground/testground/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestGroupBuildKey(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "repeated"},
			{ID: "another-id"},
			{
				ID: "custom-selector",
				BuildableComposition: BuildableComposition{
					Build: &Build{
						Selectors: []string{"a", "b"},
					},
				},
			},
			{
				ID: "duplicate-selector",
				BuildableComposition: BuildableComposition{
					Build: &Build{
						Selectors: []string{"a", "b"},
					},
				},
			},
			{
				ID: "duplicate-selector-with-different-build-config",

				BuildableComposition: BuildableComposition{
					Build: &Build{
						Selectors: []string{"a", "b"},
					},
					BuildConfig: map[string]interface{}{
						"dockerfile_extensions": map[string]string{
							"pre_mod_download": "pre_mod_download_overriden",
						},
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

func TestGroupBuildKeyWithCustomPlanAndBuilder(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "repeated"},
			{ID: "another-id"},
			{
				ID: "custom-plan",
				BuildableComposition: BuildableComposition{
					Plan: "another_plan",
				},
			},
			{
				ID: "custom-builder",
				BuildableComposition: BuildableComposition{
					Case:    "foo_case",
					Builder: "docker:generic",
				},
			},
		},
	}

	k0 := c.Groups[0].BuildKey() // repeated
	k1 := c.Groups[1].BuildKey() // another-id

	k2 := c.Groups[2].BuildKey() // custom-plan
	k3 := c.Groups[3].BuildKey() // custom-builder

	require.EqualValues(t, k0, k1)
	require.NotEqualValues(t, k0, k2)
	require.NotEqualValues(t, k2, k3)
	require.NotEqualValues(t, k0, k3)
}

func TestDefaultTestParamsApplied(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			TotalInstances: 3,
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
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
				Build: &Build{
					Selectors: []string{"default_selector_1", "default_selector_2"},
					Dependencies: []Dependency{
						{"dependency:a", "", "1.0.0.default"},
						{"dependency:b", "", "2.0.0.default"},
					},
				},
			},
			TotalInstances: 3,
			Runner:         "local:docker",
		},
		Groups: []*Group{
			{
				ID: "no_local_settings",
			},
			{
				ID: "dep_override",
				BuildableComposition: BuildableComposition{
					Build: &Build{
						Dependencies: []Dependency{
							{"dependency:a", "", "1.0.0.overridden"},
							{"dependency:c", "", "1.0.0.locally_set"},
							{"dependency:d", "remote/fork", "1.0.0.locally_set"},
						},
					},
				},
			},
			{
				ID: "selector_and_dep_override",

				BuildableComposition: BuildableComposition{
					Build: &Build{
						Selectors: []string{"overridden"},
						Dependencies: []Dependency{
							{"dependency:a", "", "1.0.0.overridden"},
							{"dependency:c", "", "1.0.0.locally_set"},
						},
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
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
				BuildConfig: map[string]interface{}{
					"build_base_image": "base_image_global",
				},
			},
			TotalInstances: 3,
			Runner:         "local:docker",
		},
		Groups: []*Group{
			{
				ID: "no_local_settings",
			},
			{
				ID: "dockerfile_override",
				BuildableComposition: BuildableComposition{
					BuildConfig: map[string]interface{}{
						"dockerfile_extensions": map[string]string{
							"pre_mod_download": "pre_mod_download_overriden",
						},
					},
				},
			},
			{
				ID: "build_base_image_override",
				BuildableComposition: BuildableComposition{
					BuildConfig: map[string]interface{}{
						"build_base_image": "base_image_overriden",
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
}

func TestPrepareForBuildOnGroupTrickleConfigurationFromGlobal(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "first-group"},
			{
				ID: "custom-build",
				BuildableComposition: BuildableComposition{
					Plan:    "another_plan",
					Case:    "another_case",
					Builder: "docker:generic",
				},
			},
			{
				ID: "third-group",
				BuildableComposition: BuildableComposition{
					Plan: "alternative_plan",
				},
			},
		},
	}

	manifest1 := &TestPlanManifest{
		Name: "foo_plan:go",
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}
	manifest2 := &TestPlanManifest{
		Name: "foo_plan:generic",
		Builders: map[string]config.ConfigMap{
			"docker:generic": {},
		},
	}

	g1, err1 := c.Groups[0].PrepareForBuild(&c.Global, manifest1)
	g2, err2 := c.Groups[1].PrepareForBuild(&c.Global, manifest2)
	g3, err3 := c.Groups[2].PrepareForBuild(&c.Global, manifest1)

	require.Nil(t, err1)
	require.Nil(t, err2)
	require.Nil(t, err3)

	require.EqualValues(t, "foo_plan:go", g1.Plan)
	require.EqualValues(t, "foo_case", g1.Case)
	require.EqualValues(t, "docker:go", g1.Builder)

	require.EqualValues(t, "foo_plan:generic", g2.Plan)
	require.EqualValues(t, "another_case", g2.Case)
	require.EqualValues(t, "docker:generic", g2.Builder)

	require.EqualValues(t, "foo_plan:go", g3.Plan)
	require.EqualValues(t, "foo_case", g3.Case)
	require.EqualValues(t, "docker:go", g3.Builder)
}

func TestPrepareForBuildOnGroup(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "first-group"},
			{
				ID: "custom-build",
				BuildableComposition: BuildableComposition{
					Plan:    "another_plan",
					Case:    "another_case",
					Builder: "docker:generic",
				},
			},
			{
				ID: "third-group",
				BuildableComposition: BuildableComposition{
					Plan: "alternative_plan",
				},
			},
		},
	}

	manifest1 := &TestPlanManifest{
		Name: "foo_plan::manifest",
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}
	manifest2 := &TestPlanManifest{
		Name: "another_plan::manifest",
		Builders: map[string]config.ConfigMap{
			"docker:generic": {},
		},
	}

	g1, err1 := c.Groups[0].PrepareForBuild(&c.Global, manifest1)
	g2, err2 := c.Groups[1].PrepareForBuild(&c.Global, manifest2)
	g3, err3 := c.Groups[2].PrepareForBuild(&c.Global, manifest1)

	require.Nil(t, err1)
	require.Nil(t, err2)
	require.Nil(t, err3)

	require.EqualValues(t, "foo_plan::manifest", g1.Plan)
	require.EqualValues(t, "foo_case", g1.Case)
	require.EqualValues(t, "docker:go", g1.Builder)

	require.EqualValues(t, "another_plan::manifest", g2.Plan)
	require.EqualValues(t, "another_case", g2.Case)
	require.EqualValues(t, "docker:generic", g2.Builder)

	require.EqualValues(t, "foo_plan::manifest", g3.Plan)
	require.EqualValues(t, "foo_case", g3.Case)
	require.EqualValues(t, "docker:go", g3.Builder)
}

func TestPrepareForBuildOnGroupAppliesBuildConfiguration(t *testing.T) {
	manifest := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go": {
				"manifest_build_config": "value0",
			},
		},
	}

	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
				BuildConfig: map[string]interface{}{
					"global_build_config": "value1",
				},
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "first-group"},
			{
				ID: "custom-build",
				BuildableComposition: BuildableComposition{
					BuildConfig: map[string]interface{}{
						"group_build_config": "value2",
					},
				},
			},
			{
				ID:                   "third-group",
				BuildableComposition: BuildableComposition{},
			},
		},
	}

	g1, err1 := c.Groups[0].PrepareForBuild(&c.Global, manifest)
	g2, err2 := c.Groups[1].PrepareForBuild(&c.Global, manifest)
	g3, err3 := c.Groups[2].PrepareForBuild(&c.Global, manifest)

	require.Nil(t, err1)
	require.Nil(t, err2)
	require.Nil(t, err3)

	require.EqualValues(t, "value0", g1.BuildConfig["manifest_build_config"])
	require.EqualValues(t, "value1", g1.BuildConfig["global_build_config"])
	require.EqualValues(t, nil, g1.BuildConfig["group_build_config"])

	require.EqualValues(t, "value0", g2.BuildConfig["manifest_build_config"])
	require.EqualValues(t, "value1", g2.BuildConfig["global_build_config"])
	require.EqualValues(t, "value2", g2.BuildConfig["group_build_config"])

	require.EqualValues(t, "value0", g3.BuildConfig["manifest_build_config"])
	require.EqualValues(t, "value1", g3.BuildConfig["global_build_config"])
	require.EqualValues(t, nil, g3.BuildConfig["group_build_config"])
}

func TestPrepareForBuildOnGroupAppliesBuildConfigurationWithNilValue(t *testing.T) {
	manifest := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}

	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
				BuildConfig: map[string]interface{}{
					"global_build_config": "value1",
				},
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{
				ID: "custom-build",
				BuildableComposition: BuildableComposition{
					BuildConfig: map[string]interface{}{
						"group_build_config": "value2",
					},
				},
			},
		},
	}

	g1, err1 := c.Groups[0].PrepareForBuild(&c.Global, manifest)

	require.Nil(t, err1)

	require.EqualValues(t, nil, g1.BuildConfig["manifest_build_config"])
	require.EqualValues(t, "value1", g1.BuildConfig["global_build_config"])
	require.EqualValues(t, "value2", g1.BuildConfig["group_build_config"])
}

func TestPrepareForBuildOnGroupAppliesBuild(t *testing.T) {
	manifest := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}

	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
				Build: &Build{
					Selectors: []string{"foo", "bar"},
					Dependencies: []Dependency{
						{
							Module:  "baz",
							Version: "1",
						},
						{
							Module:  "qux",
							Version: "11",
						},
					},
				},
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "first-group"},
			{
				ID: "custom-build",
				BuildableComposition: BuildableComposition{
					Build: &Build{
						Selectors: []string{"overwritten"},
					},
				},
			},
			{
				ID: "third-group",
				BuildableComposition: BuildableComposition{
					Build: &Build{
						Dependencies: []Dependency{
							{
								Module:  "baz",
								Version: "2",
							},
							{
								Module:  "pok",
								Version: "9",
							},
						},
					},
				},
			},
		},
	}

	g1, err1 := c.Groups[0].PrepareForBuild(&c.Global, manifest)
	g2, err2 := c.Groups[1].PrepareForBuild(&c.Global, manifest)
	g3, err3 := c.Groups[2].PrepareForBuild(&c.Global, manifest)

	require.Nil(t, err1)
	require.Nil(t, err2)
	require.Nil(t, err3)

	require.EqualValues(t, c.Global.Build.Selectors, g1.Build.Selectors)
	require.EqualValues(t, c.Global.Build.Dependencies, g1.Build.Dependencies)

	require.EqualValues(t, c.Groups[1].Build.Selectors, g2.Build.Selectors)
	require.EqualValues(t, c.Global.Build.Dependencies, g2.Build.Dependencies)

	require.EqualValues(t, c.Global.Build.Selectors, g3.Build.Selectors)

	expectedVersions := map[string]string{
		"baz": "2",
		"qux": "11",
		"pok": "9",
	}

	require.EqualValues(t, expectedVersions, g3.Build.Dependencies.AsMap())
}

func TestPrepareForBuildVerifiesSupportedBuilders(t *testing.T) {
	global := Global{
		BuildableComposition: BuildableComposition{
			Plan: "foo_plan",
			Case: "foo_case",
		},
		Runner: "local:docker",
	}

	manifestGo := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}

	manifestGeneric := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:generic": {},
		},
	}

	groupGo := Group{
		ID: "using-builder-go",
		BuildableComposition: BuildableComposition{
			Builder: "docker:go",
		},
	}

	groupGeneric := Group{
		ID: "using-builder-generic",
		BuildableComposition: BuildableComposition{
			Builder: "docker:generic",
		},
	}

	// Compatible Manifest
	_, err := groupGo.PrepareForBuild(&global, manifestGo)
	require.Nil(t, err)
	_, err = groupGeneric.PrepareForBuild(&global, manifestGeneric)
	require.Nil(t, err)

	// Incompatible Manifest
	_, err = groupGo.PrepareForBuild(&global, manifestGeneric)
	require.NotNil(t, err)
	_, err = groupGeneric.PrepareForBuild(&global, manifestGo)
	require.NotNil(t, err)
}

func TestBuildersList(t *testing.T) {
	m := &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go": {},
		},
	}
	require.EqualValues(t, []string{"docker:go"}, m.BuildersList())

	// Empty list
	m = &TestPlanManifest{
		Builders: map[string]config.ConfigMap{},
	}
	require.EqualValues(t, []string{}, m.BuildersList())

	// Nil builders
	m = &TestPlanManifest{
		Builders: nil,
	}
	require.EqualValues(t, []string{}, m.BuildersList())

	// Sorted builders
	m = &TestPlanManifest{
		Builders: map[string]config.ConfigMap{
			"docker:go":      {},
			"exec:go":        {},
			"docker:generic": {},
		},
	}
	require.EqualValues(t, []string{"docker:generic", "docker:go", "exec:go"}, m.BuildersList())
}

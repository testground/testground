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

	k1 := g1.BuildKey()
	k2 := g2.BuildKey()

	require.NotEqualValues(t, k1, k2)
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

	// trickle down global
	require.EqualValues(t, map[string]string{"pre_mod_download": "base_pre_mod_download"}, ret.Global.BuildConfig["dockerfile_extensions"])
	require.EqualValues(t, "base_image_global", ret.Global.BuildConfig["build_base_image"])

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

func TestGroupsMayDefineBuilder(t *testing.T) {
	g := &Group{
		ID:      "foo",
		Builder: "docker:generic",
	}

	require.NotNil(t, g)
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

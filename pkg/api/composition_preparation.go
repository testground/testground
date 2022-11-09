package api

import (
	"encoding/json"
	"fmt"
	"sort"
)

// CompleteFromUserInputs
func (c Composition) CompleteFromUserInputs() (Composition, error) {
	return Composition{}, nil
}

// PrepareForBuild verifies that this composition is compatible with
// the provided manifest for the purposes of a build, and applies any manifest-
// mandated defaults for the builder configuration.
//
// This method doesn't modify the composition, it returns a new one.
func (c Composition) PrepareForBuild(manifest *TestPlanManifest) (*Composition, error) {
	// override the composition plan name with what's in the manifest
	// rationale: composition.Global.Plan will be a path relative to
	// $TESTGROUND_HOME/plans; the server doesn't care about our local
	// paths.
	c.Global.Plan = manifest.Name

	// Is the builder supported?
	if manifest.Builders == nil || len(manifest.Builders) == 0 {
		return nil, fmt.Errorf("plan supports no builders; review the manifest")
	}

	// Apply manifest-mandated build configuration.
	if bcfg, ok := manifest.Builders[c.Global.Builder]; ok {
		if c.Global.BuildConfig == nil {
			c.Global.BuildConfig = make(map[string]interface{})
		}
		for k, v := range bcfg {
			// Apply parameters that are not explicitly set in the Composition.
			if _, ok := c.Global.BuildConfig[k]; !ok {
				c.Global.BuildConfig[k] = v
			}
		}
	}

	// Trickle global build defaults to groups, if any.
	if def := c.Global.Build; def != nil {
		for _, grp := range c.Groups {
			grp.Build.Dependencies = grp.Build.Dependencies.ApplyDefaults(def.Dependencies)
			if len(grp.Build.Selectors) == 0 {
				grp.Build.Selectors = def.Selectors
			}
		}
	}

	// Trickle global build config to groups, if any.
	if len(c.Global.BuildConfig) > 0 {
		for _, grp := range c.Groups {
			if grp.BuildConfig == nil {
				grp.BuildConfig = make(map[string]interface{})
			}

			for k, v := range c.Global.BuildConfig {
				// Note: we only merge root values.
				if _, ok := grp.BuildConfig[k]; !ok {
					grp.BuildConfig[k] = v
				}
			}
		}
	}

	// Trickle builder configuration
	for _, grp := range c.Groups {
		if grp.Builder == "" {
			grp.Builder = c.Global.Builder
		}

		if !manifest.HasBuilder(grp.Builder) {
			return nil, fmt.Errorf("plan does not support builder '%s'; supported: %v", c.Global.Builder, manifest.SupportedBuilders())
		}
	}

	return &c, nil
}

// PrepareForRun verifies that this composition is compatible with the
// provided manifest for the purposes of a run, verifies the instance count is
// within bounds, applies any manifest-mandated defaults for the runner
// configuration, and applies default run parameters.
//
// This method doesn't modify the composition, it returns a new one.
func (c Composition) PrepareForRun(manifest *TestPlanManifest) (*Composition, error) {
	// override the composition plan name with what's in the manifest
	// rationale: composition.Global.Plan will be a path relative to
	// $TESTGROUND_HOME/plans; the server doesn't care about our local
	// paths.
	c.Global.Plan = manifest.Name

	// validate the test case exists.
	_, tcase, ok := manifest.TestCaseByName(c.Global.Case)
	if !ok {
		return nil, fmt.Errorf("test case %s not found in plan %s", c.Global.Case, manifest.Name)
	}

	// Is the runner supported?
	if manifest.Runners == nil || len(manifest.Runners) == 0 {
		return nil, fmt.Errorf("plan supports no runners; review the manifest")
	}
	runners := make([]string, 0, len(manifest.Runners))
	for k := range manifest.Runners {
		runners = append(runners, k)
	}
	sort.Strings(runners)
	if sort.SearchStrings(runners, c.Global.Runner) == len(runners) {
		return nil, fmt.Errorf("plan does not support runner %s; supported: %v", c.Global.Runner, runners)
	}

	// Apply manifest-mandated run configuration.
	if rcfg, ok := manifest.Runners[c.Global.Runner]; ok {
		if c.Global.RunConfig == nil {
			c.Global.RunConfig = make(map[string]interface{})
		}
		for k, v := range rcfg {
			// Apply parameters that are not explicitly set in the Composition.
			if _, ok := c.Global.RunConfig[k]; !ok {
				c.Global.RunConfig[k] = v
			}
		}
	}

	// Validate the desired number of instances is within bounds.
	if t := int(c.Global.TotalInstances); t < tcase.Instances.Minimum || t > tcase.Instances.Maximum {
		str := "total instance count (%d) outside of allowable range [%d, %d] for test case %s"
		err := fmt.Errorf(str, t, tcase.Instances.Minimum, tcase.Instances.Maximum, tcase.Name)
		return nil, err
	}

	// Trickle global run defaults to groups, if any.
	if def := c.Global.Run; def != nil {
		for _, grp := range c.Groups {
			// Artifact. If a global artifact is provided, it will be applied
			// to all groups that do not set an artifact explicitly.
			// TODO(rk): this rather extreme; we might want a way to force
			//  builds for groups that do not have an artifact, even in the
			//  presence of a default one.
			if grp.Run.Artifact == "" {
				grp.Run.Artifact = def.Artifact
			}

			trickleMap := func(from, to map[string]string) (result map[string]string) {
				if to == nil {
					// copy all params in to.
					result = make(map[string]string, len(from))
					for k, v := range from {
						result[k] = v
					}
				} else {
					result = to
					// iterate over all global params, and copy over those that haven't been overridden.
					for k, v := range from {
						if _, present := to[k]; !present {
							result[k] = v
						}
					}
				}
				return result
			}

			grp.Run.TestParams = trickleMap(def.TestParams, grp.Run.TestParams)
			grp.Run.Profiles = trickleMap(def.Profiles, grp.Run.Profiles)
		}
	}

	// Apply test case param defaults. First parse all defaults as JSON data
	// types; then iterate through all the groups in the composition, and apply
	// the parameters that are absent.
	defaults := make(map[string]string, len(tcase.Parameters))
	for n, v := range tcase.Parameters {
		switch dv := v.Default.(type) {
		case string:
			defaults[n] = dv
		default:
			data, err := json.Marshal(v.Default)
			if err != nil {
				return nil, fmt.Errorf("failed to parse test case parameter; ignoring; name=%s, value=%v, err=%w", n, v, err)
			}
			defaults[n] = string(data)
		}
	}

	for _, g := range c.Groups {
		m := g.Run.TestParams
		if m == nil {
			m = make(map[string]string, len(defaults))
			g.Run.TestParams = m
		}
		for k, v := range defaults {
			if _, ok := m[k]; !ok {
				m[k] = v
			}
		}
	}

	return &c, nil
}
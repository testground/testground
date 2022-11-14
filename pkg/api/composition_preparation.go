package api

import (
	"fmt"
	"math"

	"github.com/imdario/mergo"
)

// PrepareForBuild verifies that this group is compatible with
// the provided manifest for the purposes of a build, and applies any manifest
// and global compositions defaults for the builder configuration.
//
// TODO: the build config mapping is not deep-cloned, fix.
// This method doesn't modify the Group, it returns a new one.
func (g Group) PrepareForBuild(manifest *TestPlanManifest, c *Composition) (*Group, error) {
	// trickle down builder
	if g.Builder == "" {
		g.Builder = c.Global.Builder
	}

	if !manifest.HasBuilder(g.Builder) {
		return nil, fmt.Errorf("plan does not support builder '%s'; supported: %v", c.Global.Builder, manifest.SupportedBuilders())
	}

	// prepare build configuration
	if g.BuildConfig == nil {
		g.BuildConfig = make(map[string]interface{})
	}

	// load default build configuration from global
	for k, v := range c.Global.BuildConfig {
		if _, ok := g.BuildConfig[k]; !ok {
			g.BuildConfig[k] = v
		}
	}

	// load default build configuration from manifest for this builder
	if bcfg, ok := manifest.Builders[g.Builder]; ok {
		for k, v := range bcfg {
			if _, ok := g.BuildConfig[k]; !ok {
				g.BuildConfig[k] = v
			}
		}
	}

	// Prepare build field: trickle global build defaults to groups, if any.
	if def := c.Global.Build; def != nil {
		g.Build.Dependencies = g.Build.Dependencies.ApplyDefaults(def.Dependencies)
		if len(g.Build.Selectors) == 0 {
			g.Build.Selectors = def.Selectors
		}
	}

	return &g, nil
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

	// Require a builders in the manifest.
	if manifest.Builders == nil || len(manifest.Builders) == 0 {
		return nil, fmt.Errorf("plan supports no builders; review the manifest")
	}

	// Default groups configuration from the Global configuration + Manifest
	newGroups := make(Groups, len(c.Groups))
	for i, g := range c.Groups {
		newGroup, err := g.PrepareForBuild(manifest, &c)

		if err != nil {
			return nil, fmt.Errorf("error preparing group %s: %w", g.ID, err)
		}

		newGroups[i] = newGroup
	}
	c.Groups = newGroups

	return &c, nil
}

// Generate Default Run
// This method doesn't modify the composition, it returns a new one.
func (c Composition) GenerateDefaultRun() *Composition {
	// Generate Default Run
	if len(c.Runs) == 0 {
		r := Run{
			ID:             "default",
			TotalInstances: c.Global.TotalInstances,
			Groups:         CompositionRunGroups{},
		}

		for _, g := range c.Groups {
			r.Groups = append(r.Groups, g.DefaultRunGroup())
		}

		c.Runs = Runs{&r}
	}

	return &c
}

// PrepareForRun verifies that this composition is compatible with the
// provided manifest for the purposes of a run, verifies the instance count is
// within bounds, applies any manifest-mandated defaults for the runner
// configuration, and applies default run parameters.
//
// This method doesn't modify the composition, it returns a new one.
func (c Composition) PrepareForRun(manifest *TestPlanManifest) (*Composition, error) {
	c = *c.GenerateDefaultRun()
	
	// override the composition plan name with what's in the manifest
	// rationale: composition.Global.Plan will be a path relative to
	// $TESTGROUND_HOME/plans; the server doesn't care about our local
	// paths.
	c.Global.Plan = manifest.Name

	// validate the test case exists.
	_, _, ok := manifest.TestCaseByName(c.Global.Case)
	if !ok {
		return nil, fmt.Errorf("test case %s not found in plan %s", c.Global.Case, manifest.Name)
	}

	// Require a runner in the manifest.
	if manifest.Runners == nil || len(manifest.Runners) == 0 {
		return nil, fmt.Errorf("plan supports no runners; review the manifest")
	}

	// Is the runner supported?
	if !manifest.HasRunner(c.Global.Runner) {
		return nil, fmt.Errorf("plan does not support runner '%s'; supported: %v", c.Global.Runner, manifest.SupportedRunners())
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

	// Default runs configuration from the Global configuration + Manifest
	newRuns := make(Runs, len(c.Runs))
	for i, r := range c.Runs {
		newRun, err := r.PrepareForRun(manifest, &c)
		if err != nil {
			return nil, fmt.Errorf("error preparing run %s: %w", r.ID, err)
		}

		newRuns[i] = newRun
	}
	c.Runs = newRuns

	return &c, nil
}

// Mutation!
func (r *Run) recalculateInstanceCounts() (error) {
	// Compute instance counts
	hasTotalInstance := r.TotalInstances != 0
	computedTotal := uint(0)

	for _, g := range r.Groups {
		// When a percentage is specified, we require that totalInstances is set
		if g.Instances.Percentage > 0 && !hasTotalInstance {
			return fmt.Errorf("groups count percentage requires a total_instance configuration")
		}

		if g.calculatedInstanceCnt = g.Instances.Count; g.calculatedInstanceCnt == 0 {
			g.calculatedInstanceCnt = uint(math.Round(g.Instances.Percentage * float64(r.TotalInstances)))
		}
		computedTotal += g.calculatedInstanceCnt
	}

	if hasTotalInstance && computedTotal != r.TotalInstances {
		return fmt.Errorf("total instances mismatch: computed: %d != configured: %d", computedTotal, r.TotalInstances)
	}

	r.TotalInstances = computedTotal

	return nil
}

func (r Run) PrepareForRun(manifest *TestPlanManifest, composition *Composition) (*Run, error) {
	// Prepare run groups with default values.
	newGroups := make(CompositionRunGroups, len(r.Groups))
	for i, g := range r.Groups {
		g, err := g.PrepareForRun(manifest, composition)

		if err != nil {
			return nil, err
		}

		newGroups[i] = g
	}
	r.Groups = newGroups

	err := r.recalculateInstanceCounts()
	if err != nil {
		return nil, err
	}

	// Validate the desired number of instances is within bounds.
	_, tcase, ok := manifest.TestCaseByName(composition.Global.Case)
	if !ok {
		return nil, fmt.Errorf("test case %s not found", composition.Global.Case)
	}

	if t := int(r.TotalInstances); t < tcase.Instances.Minimum || t > tcase.Instances.Maximum {
		str := "total instance count (%d) outside of allowable range [%d, %d] for test case %s"
		err := fmt.Errorf(str, t, tcase.Instances.Minimum, tcase.Instances.Maximum, tcase.Name)
		return nil, err
	}

	return &r, nil
}

func (g CompositionRunGroup) PrepareForRun(manifest *TestPlanManifest, composition *Composition) (*CompositionRunGroup, error) {
	// Merge groups defaults
	buildGroup, err := composition.GetGroup(g.EffectiveGroupId())

	if err != nil {
		return nil, err
	}

	err = g.merge(buildGroup)
	if err != nil {
		return nil, err
	}

	// Merge global defaults
	if composition.Global.Run != nil {
		err = g.mergeRun(composition.Global.Run)
		if err != nil {
			return nil, err
		}
	}

	// Merge testcase defaults
	testParamsDefaults, err := manifest.defaultParameters(composition.Global.Case)
	if err != nil {
		return nil, err
	}

	err = mergo.Merge(&g.TestParams, testParamsDefaults)
	if err != nil {
		return nil, err
	}

	return &g, nil
}

package api

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/go-playground/validator/v10"
)

var compositionValidator = func() *validator.Validate {
	v := validator.New()
	v.RegisterStructValidation(ValidateInstances, &Instances{})
	return v
}()

type Composition struct {
	// Metadata expresses optional metadata about this composition.
	Metadata Metadata `toml:"metadata" json:"metadata"`

	// Global defines the general parameters for this composition.
	Global Global `toml:"global" json:"global"`

	// Groups enumerates the instances groups that participate in this
	// composition.
	Groups []*Group `toml:"groups" json:"groups" validate:"unique=ID"`
}

type Global struct {
	// Plan is the test plan we want to run.
	Plan string `toml:"plan" json:"plan" validate:"required"`

	// Case is the test case we want to run.
	Case string `toml:"case" json:"case" validate:"required"`

	// TotalInstances defines the total number of instances that participate in
	// this composition; it is the sum of all instances in all groups.
	TotalInstances uint `toml:"total_instances" json:"total_instances" validate:"required,gte=0"`

	// Builder is the builder we're using.
	Builder string `toml:"builder" json:"builder" validate:"required"`

	// BuildConfig specifies the build configuration for this run.
	BuildConfig map[string]interface{} `toml:"build_config" json:"build_config"`

	// Build applies global build defaults that trickle down to all groups, such
	// as selectors or dependencies. Groups can override these in their local
	// build definition.
	Build *Build `toml:"build" json:"build"`

	// Runner is the runner we're using.
	Runner string `toml:"runner" json:"runner" validate:"required"`

	// RunConfig specifies the run configuration for this run.
	RunConfig map[string]interface{} `toml:"run_config" json:"run_config"`

	// Run applies global run defaults that trickle down to all groups, such as
	// test parameters or build artifacts. Groups can override these in their
	// local run definition.
	Run *Run `toml:"run" json:"run"`
}

type Metadata struct {
	// Name is the name of this composition.
	Name string `toml:"name" json:"name"`

	// Author is the author of this composition.
	Author string `toml:"author" json:"author"`
}

type Resources struct {
	Memory string `toml:"memory" json:"memory"`
	CPU    string `toml:"cpu" json:"cpu"`
}

type Group struct {
	// ID is the unique ID of this group.
	ID string `toml:"id" json:"id"`

	// Resources requested for each pod from the Kubernetes cluster
	Resources Resources `toml:"resources" json:"resources"`

	// Instances defines the number of instances that belong to this group.
	Instances Instances `toml:"instances" json:"instances"`

	// Build specifies the build configuration for this group.
	Build Build `toml:"build" json:"build"`

	// Run specifies the run configuration for this group.
	Run Run `toml:"run" json:"run"`

	// calculatedInstanceCnt caches the actual amount of instances in this
	// group.
	calculatedInstanceCnt uint
}

// CalculatedInstanceCount returns the actual number of instances in this group.
//
// Validate MUST be called for this field to be available.
func (g *Group) CalculatedInstanceCount() uint {
	return g.calculatedInstanceCnt
}

type Instances struct {
	// Count specifies the exact number of instances that belong to a group.
	//
	// Specifying a count is mutually exclusive with specifying a percentage.
	Count uint `toml:"count" json:"count"`

	// Percentage indicates the number of instances belonging to a group as a
	// proportion of the total instance count.
	//
	// Specifying a percentage is mutually exclusive with specifying a count.
	Percentage float64 `toml:"percentage" json:"percentage"`
}

type Dependencies []Dependency

type Build struct {
	// Selectors specifies any source selection strings to be sent to the
	// builder. In the case of go builders, this field maps to build tags.
	Selectors []string `toml:"selectors" json:"selectors"`

	// Dependencies specifies any upstream dependency overrides to apply to this
	// build.
	Dependencies Dependencies `toml:"dependencies" json:"dependencies"`
}

// BuildKey returns a composite key that identifies this build, suitable for
// deduplication.
func (b Build) BuildKey() string {
	var sb strings.Builder

	// canonicalise selectors.
	selectors := append(b.Selectors[:0:0], b.Selectors...)
	sort.Strings(selectors)
	sb.WriteString(fmt.Sprintf("selectors=%s;", strings.Join(selectors, ",")))

	// canonicalise dependencies.
	dependencies := append(b.Dependencies[:0:0], b.Dependencies...)
	sort.SliceStable(dependencies, func(i, j int) bool {
		return strings.Compare(dependencies[i].Module, dependencies[j].Module) < 0
	})
	sb.WriteString("dependencies=")
	for _, d := range dependencies {
		sb.WriteString(fmt.Sprintf("%s:%s|", d.Module, d.Version))
	}

	return sb.String()
}

func (d Dependencies) AsMap() map[string]string {
	m := make(map[string]string, len(d))
	for _, dep := range d {
		m[dep.Module] = dep.Version
	}
	return m
}

// ApplyDefaults applies defaults from the provided set, only for those keys
// that are not explicitly set in the receiver.
func (d Dependencies) ApplyDefaults(defaults Dependencies) Dependencies {
	if len(d) == 0 {
		return defaults
	}

	ret := make(Dependencies, len(d), len(d)+len(defaults))
	copy(ret[:], d)

	into := d.AsMap()
	for mod, ver := range defaults.AsMap() {
		if _, present := into[mod]; !present {
			ret = append(ret, Dependency{
				Module:  mod,
				Version: ver,
			})
		}
	}
	return ret
}

type Run struct {
	// Artifact specifies the build artifact to use for this run.
	Artifact string `toml:"artifact" json:"artifact"`

	// TestParams specify the test parameters to pass down to instances of this
	// group.
	TestParams map[string]string `toml:"test_params" json:"test_params"`
}

type Dependency struct {
	// Module is the module name/path for the import to be overridden.
	Module string `toml:"module" json:"module" validate:"required"`

	// Version is the override version.
	Version string `toml:"version" json:"version" validate:"required"`
}

// ValidateForBuild validates that this Composition is correct for a build.
func (c *Composition) ValidateForBuild() error {
	return compositionValidator.StructExcept(c,
		"Global.Case",
		"Global.TotalInstances",
		"Global.Runner",
	)
}

// ValidateForRun validates that this Composition is correct for a run.
func (c *Composition) ValidateForRun() error {
	// Perform structural validation.
	if err := compositionValidator.Struct(c); err != nil {
		return err
	}

	// Calculate instances per group, and assert that sum total matches the
	// expected value.
	total, cum := c.Global.TotalInstances, uint(0)
	for i := range c.Groups {
		g := c.Groups[i]
		if g.calculatedInstanceCnt = g.Instances.Count; g.calculatedInstanceCnt == 0 {
			g.calculatedInstanceCnt = uint(math.Round(g.Instances.Percentage * float64(total)))
		}
		cum += g.calculatedInstanceCnt
	}

	if total != cum {
		return fmt.Errorf("sum of calculated instances per group doesn't match total; total=%d, calculated=%d", total, cum)
	}

	return nil
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
	builders := make([]string, 0, len(manifest.Builders))
	for k := range manifest.Builders {
		builders = append(builders, k)
	}
	sort.Strings(builders)
	if sort.SearchStrings(builders, c.Global.Builder) == len(builders) {
		return nil, fmt.Errorf("plan does not support builder %s; supported: %v", c.Global.Builder, builders)
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

			// If we have default parameters to set, handle the case where
			// the group map is uninitialized by copying over all defaults,
			// as well as the case where we have to merge.
			if len(def.TestParams) > 0 {
				if grp.Run.TestParams == nil {
					grp.Run.TestParams = make(map[string]string, len(def.TestParams))
					for k, v := range def.TestParams {
						grp.Run.TestParams[k] = v
					}
				} else {
					// Test params.
					for k, v := range def.TestParams {
						if _, present := grp.Run.TestParams[k]; !present {
							grp.Run.TestParams[k] = v
						}
					}
				}
			}
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

// PickGroups clones this composition, retaining only the specified groups.
func (c Composition) PickGroups(indices ...int) (Composition, error) {
	for _, i := range indices {
		if i >= len(c.Groups) {
			return Composition{}, fmt.Errorf("invalid group index %d", i)
		}
	}

	grps := make([]*Group, 0, len(indices))
	for _, i := range indices {
		grps = append(grps, c.Groups[i])
	}

	// c is a value, so the receiver won't be mutated.
	c.Groups = grps
	return c, nil
}

// ValidateInstances validates that either count or percentage is provided, but
// not both.
func ValidateInstances(sl validator.StructLevel) {
	instances := sl.Current().Interface().(Instances)

	if (instances.Count == 0 || instances.Percentage == 0) && (float64(instances.Count)+instances.Percentage > 0) {
		return
	}

	sl.ReportError(instances.Count, "count", "Count", "count_or_percentage", "")
	sl.ReportError(instances.Percentage, "percentage", "Percentage", "count_or_percentage", "")
}

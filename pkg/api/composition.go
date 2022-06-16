package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Groups []*Group

type Composition struct {
	// Metadata expresses optional metadata about this composition.
	Metadata Metadata `toml:"metadata" json:"metadata"`

	// Global defines the general parameters for this composition.
	Global Global `toml:"global" json:"global"`

	// Groups enumerates the instances groups that participate in this
	// composition.
	Groups Groups `toml:"groups" json:"groups" validate:"required,gt=0"`
}

// TODO: find a better name
type BuildableComposition struct {
	// Plan is the test plan we want to run.
	Plan string `toml:"plan" json:"plan"`

	// Case is the test case we want to run.
	Case string `toml:"case" json:"case"`

	// Builder is the builder we're using.
	Builder string `toml:"builder" json:"builder"`

	// BuildConfig specifies the build configuration for this run.
	BuildConfig map[string]interface{} `toml:"build_config" json:"build_config"`

	// Build applies global build defaults that trickle down to all groups, such
	// as selectors or dependencies. Groups can override these in their local
	// build definition.
	Build *Build `toml:"build" json:"build"`
}

type Global struct {
	BuildableComposition

	// TotalInstances defines the total number of instances that participate in
	// this composition; it is the sum of all instances in all groups.
	TotalInstances uint `toml:"total_instances" json:"total_instances" validate:"required,gte=0"`

	// Runner is the runner we're using.
	Runner string `toml:"runner" json:"runner" validate:"required"`

	// RunConfig specifies the run configuration for this run.
	RunConfig map[string]interface{} `toml:"run_config" json:"run_config"`

	// Run applies global run defaults that trickle down to all groups, such as
	// test parameters or build artifacts. Groups can override these in their
	// local run definition.
	Run *Run `toml:"run" json:"run"`

	// DisableMetrics is used to disable metrics batching.
	DisableMetrics bool `toml:"disable_metrics" json:"disable_metrics"`
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

	BuildableComposition

	// Resources requested for each pod from the Kubernetes cluster
	Resources Resources `toml:"resources" json:"resources"`

	// Instances defines the number of instances that belong to this group.
	Instances Instances `toml:"instances" json:"instances"`

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
func (g Group) BuildKey() string {
	return g.BuildableComposition.BuildKey()
}

func (b BuildableComposition) BuildKey() string {
	data := struct {
		BuildConfig map[string]interface{} `json:"build_config"`
		BuildAsKey  string                 `json:"build_as_key"`
	}{BuildConfig: b.BuildConfig, BuildAsKey: b.Build.BuildKey()}

	j, err := json.Marshal(data)

	if err != nil {
		panic(err) // TODO: Handle better
	}

	return string(j)
}

// BuildKey returns a composite key that identifies this build, suitable for
// deduplication.
func (b *Build) BuildKey() string {
	if b == nil {
		return ""
	}

	var sb strings.Builder

	// canonicalise selectors
	// (it sorts them because when it comes to selectors [a, b] == [b, a])
	selectors := append(b.Selectors[:0:0], b.Selectors...)
	sort.Strings(selectors)
	sb.WriteString(fmt.Sprintf("selectors=%s;", strings.Join(selectors, ",")))

	// canonicalise dependencies.
	// (similarly, it sorts the dependencies)
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

	// Profiles specifies the profiles to capture, and the frequency of capture
	// of each. Profile support is SDK-dependent, as it relies entirely on the
	// facilities provided by the language runtime.
	//
	// In the case of Go, all profile kinds listed in https://golang.org/pkg/runtime/pprof/#Profile
	// are supported, taking a frequency expressed in time.Duration string
	// representation (e.g. 5s for every five seconds). Additionally, a special
	// profile kind "cpu" is supported; it takes no frequency and it starts a
	// CPU profile for the entire duration of the test.
	Profiles map[string]string `toml:"profiles" json:"profiles"`
}

type Dependency struct {
	// Module is the module name/path for the import to be overridden.
	Module string `toml:"module" json:"module" validate:"required"`

	// Target is the override module.
	Target string `toml:"target" json:"target" validate:"target"`

	// Version is the override version.
	Version string `toml:"version" json:"version" validate:"required"`
}

func mergeBuildConfigs(configs ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, config := range configs {
		for k, v := range config {
			result[k] = v
		}
	}

	return result
}

// This method doesn't modify the group, it returns a new one.
// TODO: it assumes the configuration is correct (manifest is supported, etc). Create a ValidComposition type to be explicit about it.
// TODO: the build configuration merge is not recursive for simplicity, which means that if you change docker_extension.something, it'll overwrite all docker_extension. Maybe eventually revisit and implement per-builder merge.
func (g Group) PrepareForBuild(global *Global, manifest *TestPlanManifest) (*Group, error) {
	// override the composition plan name with what's in the manifest
	// rationale: composition.Global.Plan will be a path relative to
	// $TESTGROUND_HOME/plans; the server doesn't care about our local
	// paths.
	g.Plan = manifest.Name

	if g.Builder == "" {
		g.Builder = global.Builder
	}
	if g.Case == "" {
		g.Case = global.Case
	}

	// Validate the Builder
	// TODO: extract? Validate all the fields?
	if _, has := manifest.Builders[g.Builder]; !has {
		return nil, fmt.Errorf("group(%s): manifest %s does not support builder %s (support %v)", g.ID, manifest.Name, g.Builder, manifest.BuildersList())
	}

	// Trickle down build config, manifest build config -> global build config -> group build config.
	manifestBuildConfig := manifest.Builders[g.Builder]
	bc := mergeBuildConfigs(manifestBuildConfig, global.BuildConfig, g.BuildConfig)
	g.BuildConfig = bc

	// Trickle down build, (manifest, global, group)
	build := g.Build
	if build == nil {
		build = &Build{
			Selectors:    []string{},
			Dependencies: []Dependency{},
		}
	}
	if buildDefaults := global.Build; buildDefaults != nil {
		build.Dependencies = build.Dependencies.ApplyDefaults(buildDefaults.Dependencies)
		if len(build.Selectors) == 0 {
			build.Selectors = buildDefaults.Selectors
		}
	}
	g.Build = build

	return &g, nil
}

func (m *TestPlanManifest) BuildersList() []string {
	builders := make([]string, 0, len(m.Builders))
	for k := range m.Builders {
		builders = append(builders, k)
	}
	sort.Strings(builders)
	return builders
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
	builders := manifest.BuildersList()

	if len(builders) == 0 {
		return nil, fmt.Errorf("plan supports no builders; review the manifest")
	}
	if sort.SearchStrings(builders, c.Global.Builder) == len(builders) {
		return nil, fmt.Errorf("plan does not support builder %s; supported: %v", c.Global.Builder, builders)
	}

	// Recursive prepare for groups
	for i, g := range c.Groups {
		prepared, err := g.PrepareForBuild(&c.Global, manifest)
		if err != nil {
			return nil, err
		}
		c.Groups[i] = prepared
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

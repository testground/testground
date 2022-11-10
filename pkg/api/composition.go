package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Groups []*Group

type Runs []*Run

type Composition struct {
	// Metadata expresses optional metadata about this composition.
	Metadata Metadata `toml:"metadata" json:"metadata"`

	// Global defines the general parameters for this composition.
	Global Global `toml:"global" json:"global"`

	// Groups enumerates the instances groups that participate in this
	// composition.
	Groups Groups `toml:"groups" json:"groups" validate:"required,gt=0"`

	// Runs enumerate the runs that participate in this composition.
	Runs Runs `toml:"runs" json:"runs" validate:"required,gt=0"`
}

type Global struct {
	// Plan is the test plan we want to run.
	Plan string `toml:"plan" json:"plan" validate:"required"`

	// Case is the test case we want to run.
	Case string `toml:"case" json:"case" validate:"required"`

	// TotalInstances defines the total number of instances that participate in
	// this composition; it is the sum of all instances in all groups.
	TotalInstances uint `toml:"total_instances" json:"total_instances" validate:"gte=0"`

	// ConcurrentBuilds defines the maximum number of concurrent builds that are
	// scheduled for this test.
	ConcurrentBuilds int `toml:"concurrent_builds" json:"concurrent_builds"`

	// Builder is the builder we're using.
	Builder string `toml:"builder" json:"builder"`

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
	Run *RunParams `toml:"run" json:"run"`

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

	// Builder is the builder we're using.
	Builder string `toml:"builder" json:"builder"`

	// BuildConfig specifies the build configuration for this run.
	BuildConfig map[string]interface{} `toml:"build_config" json:"build_config"`

	// Build specifies the build configuration for this group.
	Build Build `toml:"build" json:"build"`

	RunnableItem
}

type Run struct {
	// ID is the unique ID of this group.
	ID string `toml:"id" json:"id"`

	// TotalInstances defines the total number of instances that participate in
	// this run; it is the sum of all instances in all groups.
	TotalInstances uint `toml:"total_instances" json:"total_instances" validate:"gte=0"`

	// Instances defines the number of instances that belong to this group.
	Groups CompositionRunGroups `toml:"groups" json:"groups" validate:"required,gt=0"`
}

type CompositionRunGroups []*CompositionRunGroup

type CompositionRunGroup struct {
	// ID is the unique ID of this group.
	ID string `toml:"id" json:"id"`

	RunnableItem
}

type RunnableItem struct {
	// Resources requested for each pod from the Kubernetes cluster
	Resources Resources `toml:"resources" json:"resources"`

	// Instances defines the number of instances that belong to this group.
	Instances Instances `toml:"instances" json:"instances"`

	// Run specifies the run configuration for this group.
	Run RunParams `toml:"run" json:"run"`

	// calculatedInstanceCnt caches the actual number of instances in this
	// group.
	calculatedInstanceCnt uint
}

// CalculatedInstanceCount returns the actual number of instances in this group.
//
// Validate MUST be called for this field to be available.
func (r *RunnableItem) CalculatedInstanceCount() uint {
	return r.calculatedInstanceCnt
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
	if g.Builder == "" {
		// NOTE: A composition can be unprepared or prepared. We assume the composition has
		// been prepared when we reach this code.
		panic("group must have a builder")
	}

	data := struct {
		Builder     string                 `json:"builder"`
		BuildConfig map[string]interface{} `json:"build_config"`
		BuildAsKey  string                 `json:"build_as_key"`
	}{Builder: g.Builder, BuildConfig: g.BuildConfig, BuildAsKey: g.Build.BuildKey()}

	j, err := json.Marshal(data)

	if err != nil {
		panic(err) // TODO: Handle better
	}

	return string(j)
}

// BuildKey returns a composite key that identifies this build, suitable for
// deduplication.
func (b Build) BuildKey() string {
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

func (d Dependencies) AsMap() map[string]Dependency {
	m := make(map[string]Dependency, len(d))
	for _, dep := range d {
		m[dep.Module] = dep
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
	for mod, dep := range defaults.AsMap() {
		if _, present := into[mod]; !present {
			ret = append(ret, Dependency{
				Module:  mod,
				Target: dep.Target,
				Version: dep.Version,
			})
		}
	}
	return ret
}

type RunParams struct {
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

func (c *Composition) ListBuilders() []string {
	builders := make(map[string]bool)

	for _, grp := range c.Groups {
		if grp.Builder == "" {
			builders[c.Global.Builder] = true
		} else {
			builders[grp.Builder] = true
		}
	}

	result := make([]string, 0, len(builders))
	for k := range builders {
		result = append(result, k)
	}

	sort.Strings(result)

	return result
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

// FrameForRuns clones this composition, retaining only the specified run ids and corresponding groups
// TODO: Verify that our composition and nested objects are immutables / deep copies.
func (c Composition) FrameForRuns(runIds ...string) (Composition, error) {
	requiredGroupsIdx := make(map[string]bool)
	runs := make([]*Run, 0, len(runIds))

	// Gather every run used + the corresponding groups.
	for _, runId := range runIds {
		run, err := c.getRun(runId)

		if err != nil {
			return Composition{}, fmt.Errorf("invalid run id %s: %w", runId, err)
		}

		for _, groupIdx := range run.Groups {
			requiredGroupsIdx[groupIdx.ID] = true
		}

		runs = append(runs, run)
	}

	// Gather the groups that we listed in requiredGroupsIdx.
	groups := make([]*Group, 0, len(requiredGroupsIdx))
	for groupId := range requiredGroupsIdx {
		group, err := c.getGroup(groupId)

		if err != nil {
			return Composition{}, fmt.Errorf("invalid group id %s: %w", groupId, err)
		}

		groups = append(groups, group)
	}

	c.Groups = groups
	c.Runs = runs

	// TODO: update the ValidateForRun
	// return c, c.ValidateForRun()

	return c, nil
}

func (c Composition) getRun(runId string) (*Run, error) {
	for _, x := range c.Runs {
		if x.ID == runId {
			return x, nil
		}
	}
	return nil, fmt.Errorf("unknown run id %s", runId)
}

func (c Composition) getGroup(groupId string) (*Group, error) {
	for _, x := range c.Groups {
		if x.ID == groupId {
			return x, nil
		}
	}
	return nil, fmt.Errorf("unknown group id %s", groupId)
}

func (c Composition) ListRunIds() []string {
	ids := make([]string, 0, len(c.Runs))
	for _, x := range c.Runs {
		ids = append(ids, x.ID)
	}
	sort.Strings(ids)
	return ids
}

func (c Composition) ListGroupsId() []string {
	ids := make([]string, 0, len(c.Groups))
	for _, x := range c.Groups {
		ids = append(ids, x.ID)
	}
	sort.Strings(ids)
	return ids
}
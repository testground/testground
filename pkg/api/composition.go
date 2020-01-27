package api

import (
	"fmt"
	"math"

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
	Groups []Group `toml:"groups" json:"groups" validate:"unique=ID"`
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

	// Runner is the runner we're using.
	Runner string `toml:"runner" json:"runner" validate:"required"`
}

type Metadata struct {
	// Name is the name of this composition.
	Name string `toml:"name" json:"name"`

	// Author is the author of this composition.
	Author string `toml:"author" json:"author"`
}

type Group struct {
	// ID is the unique ID of this group.
	ID string `toml:"id" json:"id"`

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
func (g Group) CalculatedInstanceCount() uint {
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
	// Dependencies specifies any upstream dependency overrides to apply to this
	// build.
	Dependencies Dependencies `toml:"dependencies" json:"dependencies"`

	// Configuration specifies the build configuration for this build.
	Configuration map[string]interface{} `toml:"config" json:"config"`
}

func (d Dependencies) AsMap() map[string]string {
	m := make(map[string]string, len(d))
	for _, dep := range d {
		m[dep.Module] = dep.Version
	}
	return m
}

type Run struct {
	// Artifact specifies the build artifact to use for this run.
	Artifact string `toml:"artifact" json:"artifact"`

	// Configuration specifies the run configuration for this run.
	Configuration map[string]interface{} `toml:"config" json:"config"`

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

// Validate validates that this Composition is correct.
func (c *Composition) Validate() error {
	// Perform structural validation.
	if err := compositionValidator.Struct(c); err != nil {
		return err
	}

	// Calculate instances per group, and assert that sum total matches the
	// expected value.
	total, cum := c.Global.TotalInstances, uint(0)
	for i, _ := range c.Groups {
		g := &(c.Groups[i])
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

// PickGroups clones this composition, retaining only the specified groups.
func (c Composition) PickGroups(indices ...int) (Composition, error) {
	for _, i := range indices {
		if i >= len(c.Groups) {
			return Composition{}, fmt.Errorf("invalid group index %d", i)
		}
	}

	grps := make([]Group, 0, len(indices))
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

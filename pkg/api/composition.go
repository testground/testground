package api

type Composition struct {
	// Metadata expresses optional metadata about this composition.
	Metadata Metadata `toml:"metadata" json:"metadata"`

	// Global defines the general parameters for this composition.
	Global Global `toml:"global" json:"global"`

	// Groups enumerates the instances groups that participate in this
	// composition.
	Groups []Group `toml:"groups" json:"groups"`
}

type Global struct {
	// Plan is the test plan we want to run.
	Plan string `toml:"plan" json:"plan"`

	// Case is the test case we want to run.
	Case string `toml:"case" json:"case"`

	// TotalInstances defines the total number of instances that participate in
	// this composition; it is the sum of all instances in all groups.
	TotalInstances uint `toml:"total_instances" json:"total_instances"`

	// Builder is the builder we're using.
	Builder string `toml:"builder" json:"builder"`

	// Runner is the runner we're using.
	Runner string `toml:"runner" json:"runner"`
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
}

type Instances struct {
	// Count specifies the exact number of instances that belong to a group.
	Count uint `toml:"count" json:"count"`

	// Percentage indicates the number of instances belonging to a group as a
	// proportion of the total instance count.
	Percentage float64 `toml:"percentage" json:"percentage"`
}

type Build struct {
	// Dependencies specifies any upstream dependency overrides to apply to this
	// build.
	Dependencies []Dependency `toml:"dependencies" json:"dependencies"`

	// Configuration specifies the build configuration for this build.
	Configuration map[string]string `toml:"config" json:"config"`
}

type Run struct {
	// UseBuild specifies a prebuilt build artifact to use for this run.
	UseBuild string `toml:"use_build" json:"use_build"`

	// Configuration specifies the run configuration for this run.
	Configuration map[string]string `toml:"config" json:"config"`

	// TestParams specify the test parameters to pass down to instances of this
	// group.
	TestParams map[string]string `toml:"test_plan" json:"test_plan"`
}

type Dependency struct {
	// Module is the module name/path for the import to be overridden.
	Module string `toml:"module" json:"module"`

	// Version is the override version.
	Version string `toml:"version" json:"version"`
}

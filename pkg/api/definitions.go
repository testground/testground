package api

// TestPlanDefinition represents a test plan known by the system.
// Its name must be unique within a test census.
type TestPlanDefinition struct {
	Name            string
	SourcePath      string          `toml:"source_path"`
	BuildStrategies BuildStrategies `toml:"build_strategies"`
	RunStrategies   RunStrategies   `toml:"run_strategies"`
	TestCases       []*TestCase     `toml:"testcases"`
}

// TODO: add validation methods, so that we reject empty build strategies.
type BuildStrategies struct {
	DockerGo *GoBuildStrategy `toml:"docker:go"`
	ExecGo   *GoBuildStrategy `toml:"exec:go"`
}

// TODO: add validation methods, so that we reject empty run strategies.
type RunStrategies struct {
	LocalExec    *PlaceholderRunStrategy `toml:"local:exec"`
	LocalDocker  *PlaceholderRunStrategy `toml:"local:docker"`
	ClusterNomad *PlaceholderRunStrategy `toml:"cluster:nomad"`
}

// TestCase represents a configuration for a test case known by the system.
type TestCase struct {
	Name      string
	Instances TestCaseInstances
	// Parameters are parameters passed to this test case at runtime.
	Parameters map[string]Parameter `toml:"params"`
	// Roles are roles that instances of this test case can assume.
	Roles []string
}

type GoBuildStrategy struct {
	Enabled    bool
	GoVersion  string `toml:"go_version" overridable:"yes"`
	ModulePath string `toml:"module_path" overridable:"yes"`
	ExecPkg    string `toml:"exec_pkg" overridable:"yes"`
}

type PlaceholderRunStrategy struct {
	Enabled bool
}

// Parameter is metadata about a test case parameter..
type Parameter struct {
	Type        string
	Description string `toml:"desc"`
	Unit        string
}

// TestCaseInstances expresses how many instances this test case can run.
type TestCaseInstances struct {
	Minimum int `toml:"min"`
	Maximum int `toml:"max"`
	Default int `toml:"default"`
}

// TestCaseByName returns a test case by name.
func (tp *TestPlanDefinition) TestCaseByName(name string) (seq int, tc *TestCase, ok bool) {
	for seq, tc := range tp.TestCases {
		if tc.Name == name {
			return seq, tc, true
		}
	}
	return -1, nil, false
}

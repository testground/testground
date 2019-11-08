package api

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/mitchellh/go-wordwrap"
)

// TestPlanDefinition represents a test plan known by the system.
// Its name must be unique within a test census.
type TestPlanDefinition struct {
	Name            string
	SourcePath      string               `toml:"source_path"`
	BuildStrategies map[string]ConfigMap `toml:"build_strategies"`
	RunStrategies   map[string]ConfigMap `toml:"run_strategies"`
	TestCases       []*TestCase          `toml:"testcases"`
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

type PlaceholderRunStrategy struct {
	Enabled bool
}

// Parameter is metadata about a test case parameter..
type Parameter struct {
	Type        string
	Description string `toml:"desc"`
	Unit        string
	Default     string
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

func (tp *TestPlanDefinition) Describe(w io.Writer) {
	p := func(w io.Writer, f string, a ...interface{}) {
		s := wordwrap.WrapString(fmt.Sprintf(f, a...), 120)
		fmt.Fprintln(w, s)
		fmt.Fprintln(w)
	}

	p(w, "This test plan is called %q.", tp.Name)

	p(w, "Its source code is picked up from %q.", tp.SourcePath)

	bs := func() (res []string) {
		for k := range tp.BuildStrategies {
			res = append(res, k)
		}
		return res
	}()
	p(w, "It can be built with strategies: %v.", bs)

	rs := func() (res []string) {
		for k := range tp.RunStrategies {
			res = append(res, k)
		}
		return res
	}()
	p(w, "It can be run with strategies: %v.", rs)

	p(w, "It has %d test cases.", len(tp.TestCases))
}

func (tc *TestCase) Describe(w io.Writer) {
	fmt.Fprintf(w, "- Test case: %s\n", tc.Name)
	fmt.Fprintf(w, "  Instances:\n")
	fmt.Fprintf(w, "    minimum: %d\n", tc.Instances.Minimum)
	fmt.Fprintf(w, "    maximum: %d\n", tc.Instances.Maximum)
	fmt.Fprintf(w, "    default: %d\n", tc.Instances.Default)
	fmt.Fprintf(w, "  Roles: %v\n", tc.Roles)
	fmt.Fprintf(w, "  Parameters:\n")

	tw := tabwriter.NewWriter(w, 1, 0, 1, ' ', tabwriter.Debug)
	for name, param := range tc.Parameters {
		fmt.Fprintf(tw, "    %s\t %s\t %s\t %s\t default: %v\n", name, param.Type, param.Description, param.Unit, param.Default)
	}
	tw.Flush()

	fmt.Fprintln(w)
}

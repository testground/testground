package api

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/testground/testground/pkg/config"

	"github.com/mitchellh/go-wordwrap"
)

// TestPlanManifest represents a test plan known by the system.
type TestPlanManifest struct {
	Name      string
	Builders  map[string]config.ConfigMap `toml:"builders"`
	Runners   map[string]config.ConfigMap `toml:"runners"`
	TestCases []*TestCase                 `toml:"testcases"`

	// ExtraSources allows the user to ship extra source directories to the
	// daemon so they can be considered in the build (e.g. assets, package
	// overrides, etc.), when certain builders are used.
	//
	// It's a mapping of builder => directories.
	ExtraSources map[string][]string `toml:"extra_sources"`
}

// TestCase represents a configuration for a test case known by the system.
type TestCase struct {
	Name      string
	Instances InstanceConstraints
	// Parameters that can be passed to this test case.
	Parameters map[string]Parameter `toml:"params"`
}

// Parameter is metadata about a test case parameter.
type Parameter struct {
	Type        string
	Description string `toml:"desc"`
	Unit        string
	Default     interface{}
}

// InstanceConstraints expresses how many instances this test case can run.
type InstanceConstraints struct {
	Minimum int `toml:"min"`
	Maximum int `toml:"max"`
}

// TestCaseByName returns a test case by name.
func (tp *TestPlanManifest) TestCaseByName(name string) (idx int, tc *TestCase, ok bool) {
	for idx, tc = range tp.TestCases {
		if tc.Name == name {
			return idx, tc, true
		}
	}
	return -1, nil, false
}

func (tp *TestPlanManifest) defaultParameters(testcaseName string) (map[string]string, error) {
	_, tc, ok := tp.TestCaseByName(testcaseName)

	if !ok {
		return nil, fmt.Errorf("test case %s not found", testcaseName)
	}

	// TODO: the typing system here is broken. Rethink.
	defaultsTestParams := make(map[string]string, len(tc.Parameters))
	for n, v := range tc.Parameters {
		switch dv := v.Default.(type) {
		case string:
			defaultsTestParams[n] = dv
		default:
			data, err := json.Marshal(v.Default)
			if err != nil {
				return nil, fmt.Errorf("failed to parse test case parameter; ignoring; name=%s, value=%v, err=%w", n, v, err)
			}
			defaultsTestParams[n] = string(data)
		}
	}

	return defaultsTestParams, nil
}

func (tp *TestPlanManifest) HasBuilder(name string) bool {
	for k := range tp.Builders {
		if k == name {
			return true
		}
	}
	return false
}

func (tp *TestPlanManifest) SupportedBuilders() []string {
	xs := make([]string, 0, len(tp.Builders))
	for x := range tp.Builders {
		xs = append(xs, x)
	}
	return xs
}

func (tp *TestPlanManifest) HasRunner(name string) bool {
	for k := range tp.Runners {
		if k == name {
			return true
		}
	}
	return false
}

func (tp *TestPlanManifest) SupportedRunners() []string {
	xs := make([]string, 0, len(tp.Runners))
	for x := range tp.Runners {
		xs = append(xs, x)
	}
	return xs
}

func (tp *TestPlanManifest) Describe(w io.Writer) {
	p := func(w io.Writer, f string, a ...interface{}) {
		s := wordwrap.WrapString(fmt.Sprintf(f, a...), 120)
		_, _ = fmt.Fprintln(w, s)
		_, _ = fmt.Fprintln(w)
	}

	p(w, "This test plan is called %q.", tp.Name)

	bs := func() (res []string) {
		for k := range tp.Builders {
			res = append(res, k)
		}
		return res
	}()
	p(w, "It can be built with strategies: %v.", bs)

	rs := func() (res []string) {
		for k := range tp.Runners {
			res = append(res, k)
		}
		return res
	}()
	p(w, "It can be run with strategies: %v.", rs)

	p(w, "It has %d test cases.", len(tp.TestCases))
}

func (tc *TestCase) Describe(w io.Writer) {
	_, _ = fmt.Fprintf(w, "- Test case: %s\n", tc.Name)
	_, _ = fmt.Fprintf(w, "  Instances:\n")
	_, _ = fmt.Fprintf(w, "    minimum: %d\n", tc.Instances.Minimum)
	_, _ = fmt.Fprintf(w, "    maximum: %d\n", tc.Instances.Maximum)
	_, _ = fmt.Fprintf(w, "  Parameters:\n")

	tw := tabwriter.NewWriter(w, 1, 0, 1, ' ', tabwriter.Debug)
	for name, param := range tc.Parameters {
		_, _ = fmt.Fprintf(tw, "    %s\t %s\t %s\t %s\t default: %v\n", name, param.Type, param.Description, param.Unit, param.Default)
	}
	tw.Flush()

	fmt.Fprintln(w)
}

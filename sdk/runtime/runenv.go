package runtime

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
)

type key int

const (
	EnvTestPlan               = "TEST_PLAN"
	EnvTestBranch             = "TEST_BRANCH"
	EnvTestCase               = "TEST_CASE"
	EnvTestTag                = "TEST_TAG"
	EnvTestRun                = "TEST_RUN"
	EnvTestRepo               = "TEST_REPO"
	EnvTestSubnet             = "TEST_SUBNET"
	EnvTestCaseSeq            = "TEST_CASE_SEQ"
	EnvTestSidecar            = "TEST_SIDECAR"
	EnvTestInstanceCount      = "TEST_INSTANCE_COUNT"
	EnvTestInstanceRole       = "TEST_INSTANCE_ROLE"
	EnvTestInstanceParams     = "TEST_INSTANCE_PARAMS"
	EnvTestGroupID            = "TEST_GROUP_ID"
	EnvTestGroupInstanceCount = "TEST_GROUP_INSTANCE_COUNT"
	EnvTestOutputsPath        = "TEST_ARTIFACTS"
)

type IPNet struct {
	net.IPNet
}

func (i IPNet) MarshalJSON() ([]byte, error) {
	if len(i.IPNet.IP) == 0 {
		return json.Marshal("")
	}
	return json.Marshal(i.String())
}

func (i *IPNet) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}

	i.IPNet = *ipnet
	return nil
}

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	*logger

	TestPlan    string `json:"plan"`
	TestCase    string `json:"case"`
	TestRun     string `json:"run"`
	TestCaseSeq int    `json:"seq"`

	TestRepo   string `json:"repo,omitempty"`
	TestCommit string `json:"commit,omitempty"`
	TestBranch string `json:"branch,omitempty"`
	TestTag    string `json:"tag,omitempty"`

	TestArtifacts string `json:"artifacts,omitempty"`

	TestInstanceCount  int               `json:"instances"`
	TestInstanceRole   string            `json:"role,omitempty"`
	TestInstanceParams map[string]string `json:"params,omitempty"`

	TestGroupID            string `json:"group,omitempty"`
	TestGroupInstanceCount int    `json:"group_instances,omitempty"`

	// true if the test has access to the sidecar.
	TestSidecar bool `json:"test_sidecar,omitempty"`

	// The subnet on which this test is running.
	//
	// The test instance can use this to pick an IP address and/or determine
	// the "data" network interface.
	//
	// This will be 127.1.0.0/16 when using the local exec runner.
	TestSubnet *IPNet `json:"network,omitempty"`
}

func (re *RunEnv) ToEnvVars() map[string]string {
	packParams := func(in map[string]string) string {
		arr := make([]string, 0, len(in))
		for k, v := range in {
			arr = append(arr, k+"="+v)
		}
		return strings.Join(arr, "|")
	}

	out := map[string]string{
		EnvTestSidecar:            strconv.FormatBool(re.TestSidecar),
		EnvTestPlan:               re.TestPlan,
		EnvTestBranch:             re.TestBranch,
		EnvTestCase:               re.TestCase,
		EnvTestTag:                re.TestTag,
		EnvTestRun:                re.TestRun,
		EnvTestRepo:               re.TestRepo,
		EnvTestSubnet:             re.TestSubnet.String(),
		EnvTestCaseSeq:            strconv.Itoa(re.TestCaseSeq),
		EnvTestInstanceCount:      strconv.Itoa(re.TestInstanceCount),
		EnvTestInstanceRole:       re.TestInstanceRole,
		EnvTestInstanceParams:     packParams(re.TestInstanceParams),
		EnvTestGroupID:            re.TestGroupID,
		EnvTestGroupInstanceCount: strconv.Itoa(re.TestGroupInstanceCount),
		EnvTestOutputsPath:        re.TestArtifacts,
	}

	return out
}

func unpackParams(packed string) map[string]string {
	spltparams := strings.Split(packed, "|")
	params := make(map[string]string, len(spltparams))
	for _, s := range spltparams {
		v := strings.Split(s, "=")
		if len(v) != 2 {
			continue
		}
		params[v[0]] = v[1]
	}
	return params
}

func toInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return v
}

func toBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func toNet(s string) *IPNet {
	_, ipnet, _ := net.ParseCIDR(s)
	return &IPNet{IPNet: *ipnet}
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	re, _ := ParseRunEnv(os.Environ())
	return re
}

// ParseRunEnv parses a list of environment variables into a RunEnv.
func ParseRunEnv(env []string) (*RunEnv, error) {
	m, err := ParseKeyValues(env)
	if err != nil {
		return nil, err
	}

	re := &RunEnv{
		TestSidecar:            toBool(m[EnvTestSidecar]),
		TestPlan:               m[EnvTestPlan],
		TestCase:               m[EnvTestCase],
		TestRun:                m[EnvTestRun],
		TestTag:                m[EnvTestTag],
		TestBranch:             m[EnvTestBranch],
		TestRepo:               m[EnvTestRepo],
		TestSubnet:             toNet(m[EnvTestSubnet]),
		TestCaseSeq:            toInt(m[EnvTestCaseSeq]),
		TestInstanceCount:      toInt(m[EnvTestInstanceCount]),
		TestInstanceRole:       m[EnvTestInstanceRole],
		TestInstanceParams:     unpackParams(m[EnvTestInstanceParams]),
		TestGroupID:            m[EnvTestGroupID],
		TestGroupInstanceCount: toInt(m[EnvTestGroupInstanceCount]),
		TestArtifacts:          m[EnvTestOutputsPath],
	}

	re.logger = newLogger(re)

	return re, nil
}

// IsParamSet checks if a certain parameter is set.
func (re *RunEnv) IsParamSet(name string) bool {
	_, ok := re.TestInstanceParams[name]
	return ok
}

// StringParam returns a string parameter, or "" if the parameter is not set.
func (re *RunEnv) StringParam(name string) string {
	v, ok := re.TestInstanceParams[name]
	if !ok {
		panic(fmt.Errorf("%s was not set", name))
	}
	return v
}

func (re *RunEnv) SizeParam(name string) uint64 {
	v, _ := re.TestInstanceParams[name]
	m, err := humanize.ParseBytes(v)
	if err != nil {
		panic(err)
	}
	return m
}

// IntParam returns an int parameter, or -1 if the parameter is not set or
// the conversion failed. It panics on error.
func (re *RunEnv) IntParam(name string) int {
	v, ok := re.TestInstanceParams[name]
	if !ok {
		panic(fmt.Errorf("%s was not set", name))
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return i
}

// BooleanParam returns the Boolean value of the parameter, or false if not passed
func (re *RunEnv) BooleanParam(name string) bool {
	s, _ := re.TestInstanceParams[name]
	if s == "true" {
		return true
	}
	return false
}

// StringArrayParam returns an array of string parameter, or an empty array
// if it does not exist. It panics on error.
func (re *RunEnv) StringArrayParam(name string) []string {
	a := []string{}
	re.JSONParam(name, &a)
	return a
}

// SizeArrayParam returns an array of uint64 elements which represent sizes,
// in bytes. If the response is nil, then there was an error parsing the input.
// It panics on error.
func (re *RunEnv) SizeArrayParam(name string) []uint64 {
	humanSizes := re.StringArrayParam(name)
	sizes := []uint64{}

	for _, size := range humanSizes {
		n, err := humanize.ParseBytes(size)
		if err != nil {
			panic(err)
		}
		sizes = append(sizes, n)
	}

	return sizes
}

// JSONParam unmarshals a JSON parameter in an arbitrary interface.
// It panics on error.
func (re *RunEnv) JSONParam(name string, v interface{}) {
	s, ok := re.TestInstanceParams[name]
	if !ok {
		panic(fmt.Errorf("%s was not set", name))
	}

	if err := json.Unmarshal([]byte(s), v); err != nil {
		panic(err)
	}
}

// Copied from github.com/ipfs/testground/pkg/conv, because we don't want the
// SDK to depend on that package.
func ParseKeyValues(in []string) (res map[string]string, err error) {
	res = make(map[string]string, len(in))
	for _, d := range in {
		splt := strings.Split(d, "=")
		if len(splt) < 2 {
			return nil, fmt.Errorf("invalid key-value: %s", d)
		}
		res[splt[0]] = strings.Join(splt[1:], "=")
	}
	return res, nil
}

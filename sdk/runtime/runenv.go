package runtime

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"go.uber.org/zap"
)

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
	EnvTestOutputsPath        = "TEST_OUTPUTS_PATH"
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

// RunParams encapsulates the runtime parameters for this test.
type RunParams struct {
	TestPlan    string `json:"plan"`
	TestCase    string `json:"case"`
	TestRun     string `json:"run"`
	TestCaseSeq int    `json:"seq"`

	TestRepo   string `json:"repo,omitempty"`
	TestCommit string `json:"commit,omitempty"`
	TestBranch string `json:"branch,omitempty"`
	TestTag    string `json:"tag,omitempty"`

	TestOutputsPath string `json:"outputs_path,omitempty"`

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

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	RunParams
	*logger

	MetricsPusher *push.Pusher
	unstructured  chan *os.File
	structured    chan *zap.Logger
}

// NewRunEnv constructs a runtime environment from the given runtime parameters.
func NewRunEnv(params RunParams) *RunEnv {
	re := &RunEnv{
		RunParams: params,
		MetricsPusher: push.New("http://prometheus-pushgateway:9091", "testground/plan").
			Gatherer(prometheus.NewRegistry()).
			Grouping("TestPlan", params.TestPlan).
			Grouping("TestCase", params.TestCase).
			Grouping("TestRun", params.TestRun).
			Grouping("TestGroupID", params.TestGroupID).
			Grouping("TestCaseSeq", string(params.TestCaseSeq)).
			Grouping("TestCommit", params.TestCommit).
			Grouping("TestTag", params.TestTag),
		structured:   make(chan *zap.Logger, 32),
		unstructured: make(chan *os.File, 32),
	}

	re.logger = newLogger(&re.RunParams)
	return re
}

func (re *RunEnv) Close() error {
	close(re.structured)
	close(re.unstructured)

	if l := re.logger; l != nil {
		_ = l.SLogger().Sync()
	}

	for l := range re.structured {
		_ = l.Sync() // ignore errors.
	}

	for f := range re.unstructured {
		_ = f.Close() // ignore errors.
	}
	return nil
}

func (re *RunParams) ToEnvVars() map[string]string {
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
		EnvTestOutputsPath:        re.TestOutputsPath,
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

// toNet might parse any input, so it is possible to get an error and nil return value
func toNet(s string) *IPNet {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return &IPNet{IPNet: *ipnet}
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	re, _ := ParseRunEnv(os.Environ())
	return re
}

// ParseRunParams parses a list of environment variables into a RunParams.
func ParseRunParams(env []string) (*RunParams, error) {
	m, err := ParseKeyValues(env)
	if err != nil {
		return nil, err
	}

	return &RunParams{
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
		TestOutputsPath:        m[EnvTestOutputsPath],
	}, nil
}

// ParseRunEnv parses a list of environment variables into a RunEnv.
func ParseRunEnv(env []string) (*RunEnv, error) {
	p, err := ParseRunParams(env)
	if err != nil {
		return nil, err
	}

	return NewRunEnv(*p), nil
}

// IsParamSet checks if a certain parameter is set.
func (re *RunParams) IsParamSet(name string) bool {
	_, ok := re.TestInstanceParams[name]
	return ok
}

// StringParam returns a string parameter, or "" if the parameter is not set.
func (re *RunParams) StringParam(name string) string {
	v, ok := re.TestInstanceParams[name]
	if !ok {
		panic(fmt.Errorf("%s was not set", name))
	}
	return v
}

func (re *RunParams) SizeParam(name string) uint64 {
	v := re.TestInstanceParams[name]
	m, err := humanize.ParseBytes(v)
	if err != nil {
		panic(err)
	}
	return m
}

// IntParam returns an int parameter, or -1 if the parameter is not set or
// the conversion failed. It panics on error.
func (re *RunParams) IntParam(name string) int {
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
func (re *RunParams) BooleanParam(name string) bool {
	s := re.TestInstanceParams[name]
	return s == "true"
}

// StringArrayParam returns an array of string parameter, or an empty array
// if it does not exist. It panics on error.
func (re *RunParams) StringArrayParam(name string) []string {
	a := []string{}
	re.JSONParam(name, &a)
	return a
}

// SizeArrayParam returns an array of uint64 elements which represent sizes,
// in bytes. If the response is nil, then there was an error parsing the input.
// It panics on error.
func (re *RunParams) SizeArrayParam(name string) []uint64 {
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
func (re *RunParams) JSONParam(name string, v interface{}) {
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

package runtime

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type key int

const (
	EnvTestPlan           = "TEST_PLAN"
	EnvTestBranch         = "TEST_BRANCH"
	EnvTestCase           = "TEST_CASE"
	EnvTestTag            = "TEST_TAG"
	EnvTestRun            = "TEST_RUN"
	EnvTestRepo           = "TEST_REPO"
	EnvTestCaseSeq        = "TEST_CASE_SEQ"
	EnvTestInstanceCount  = "TEST_INSTANCE_COUNT"
	EnvTestInstanceRole   = "TEST_INSTANCE_ROLE"
	EnvTestInstanceParams = "TEST_INSTANCE_PARAMS"

	// runEnvContextKey key = iota
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	TestPlan    string `json:"test_plan"`
	TestCase    string `json:"test_case"`
	TestRun     string `json:"test_run"`
	TestCaseSeq int    `json:"test_seq"`

	TestRepo   string `json:"test_repo,omitempty"`
	TestCommit string `json:"test_commit,omitempty"`
	TestBranch string `json:"test_branch,omitempty"`
	TestTag    string `json:"test_tag,omitempty"`

	TestInstanceCount  int               `json:"test_instance_count"`
	TestInstanceRole   string            `json:"test_instance_role,omitempty"`
	TestInstanceParams map[string]string `json:"test_instance_params,omitempty"`

	// TODO: we'll want different kinds of loggers.
	logger  *zap.Logger
	slogger *zap.SugaredLogger
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
		EnvTestPlan:           re.TestPlan,
		EnvTestBranch:         re.TestBranch,
		EnvTestCase:           re.TestCase,
		EnvTestTag:            re.TestTag,
		EnvTestRun:            re.TestRun,
		EnvTestRepo:           re.TestRepo,
		EnvTestCaseSeq:        strconv.Itoa(re.TestCaseSeq),
		EnvTestInstanceCount:  strconv.Itoa(re.TestInstanceCount),
		EnvTestInstanceRole:   re.TestInstanceRole,
		EnvTestInstanceParams: packParams(re.TestInstanceParams),
	}

	return out
}

func (re *RunEnv) SLogger() *zap.SugaredLogger {
	if re.slogger == nil {
		re.initLoggers()
	}
	return re.slogger
}

// Loggers returns the loggers populated from this runenv.
func (re *RunEnv) Loggers() (*zap.Logger, *zap.SugaredLogger) {
	if re.logger == nil || re.slogger == nil {
		re.initLoggers()
	}
	return re.logger, re.slogger
}

// initLoggers populates loggers from this RunEnv.
func (re *RunEnv) initLoggers() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.FatalLevel)

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		l := zapcore.Level(0)
		l.UnmarshalText([]byte(level))
		cfg.Level = zap.NewAtomicLevelAt(l)
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	re.logger = logger.With(
		zap.String("plan", re.TestPlan),
		zap.String("case", re.TestCase),
		zap.String("run", re.TestRun),
		zap.Int("seq", re.TestCaseSeq),
		zap.String("repo", re.TestRepo),
		zap.String("commit", re.TestCommit),
		zap.String("branch", re.TestBranch),
		zap.String("tag", re.TestTag),
		zap.Int("instances", re.TestInstanceCount),
	)
	re.slogger = re.logger.Sugar()
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	toInt := func(s string) int {
		v, err := strconv.Atoi(s)
		if err != nil {
			return -1
		}
		return v
	}

	unpackParams := func(packed string) map[string]string {
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

	re := &RunEnv{
		TestPlan:           os.Getenv(EnvTestPlan),
		TestCase:           os.Getenv(EnvTestCase),
		TestRun:            os.Getenv(EnvTestRun),
		TestTag:            os.Getenv(EnvTestTag),
		TestBranch:         os.Getenv(EnvTestBranch),
		TestRepo:           os.Getenv(EnvTestRepo),
		TestCaseSeq:        toInt(os.Getenv(EnvTestCaseSeq)),
		TestInstanceCount:  toInt(os.Getenv(EnvTestInstanceCount)),
		TestInstanceRole:   os.Getenv(EnvTestInstanceRole),
		TestInstanceParams: unpackParams(os.Getenv(EnvTestInstanceParams)),
	}

	re.initLoggers()

	return re
}

// StringParam returns a string parameter, or "" if the parameter is not set.
// The second return value indicates if the parameter was set.
func (re *RunEnv) StringParam(name string) (s string, ok bool) {
	v, ok := re.TestInstanceParams[name]
	return v, ok
}

// StringParamD returns a string parameter, or the default value if not set.
func (re *RunEnv) StringParamD(name string, def string) string {
	s, ok := re.StringParam(name)
	if !ok {
		return def
	}
	return s
}

// IntParam returns an int parameter, or -1 if the parameter is not set. The
// second return value indicates if the parameter was set.
func (re *RunEnv) IntParam(name string) (i int, ok bool) {
	v, ok := re.TestInstanceParams[name]
	if !ok {
		return -1, false
	}
	i, err := strconv.Atoi(v)
	return i, err == nil
}

// IntParamD returns an int parameter, or the default value if not set.
func (re *RunEnv) IntParamD(name string, def int) int {
	i, ok := re.IntParam(name)
	if !ok {
		return def
	}
	return i
}

// BooleanParam returns the Boolean value of the parameter, or false if not passed
// The second return value indicates if the parameter was set.
func (re *RunEnv) BooleanParam(name string) (b bool, ok bool) {
	s, ok := re.TestInstanceParams[name]
	if s == "true" {
		return true, ok
	} else {
		return false, ok
	}
}

// BooleanParamD returns a Boolean parameter, or the default value if not set.
func (re *RunEnv) BooleanParamD(name string, def bool) bool {
	b, ok := re.BooleanParam(name)
	if !ok {
		return def
	}
	return b
}

// // ExtractRunEnv extracts the test context from a context.Context object.
// func ExtractRunEnv(ctx context.Context) *RunEnv {
// 	c := ctx.Value(runEnvContextKey)
// 	if c == nil {
// 		panic("test context is nil")
// 	}
// 	tctx, ok := c.(*RunEnv)
// 	if !ok {
// 		panic("test context has unexpected type")
// 	}
// 	return tctx
// }

// RandomRunEnv generates a random RunEnv for testing purposes.
func RandomRunEnv() *RunEnv {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return &RunEnv{
		TestPlan:           fmt.Sprintf("testplan-%d", rand.Uint32()),
		TestCase:           fmt.Sprintf("testcase-%d", rand.Uint32()),
		TestRun:            fmt.Sprintf("testrun-%d", rand.Uint32()),
		TestCaseSeq:        int(rand.Uint32()),
		TestRepo:           "github.com/ipfs/go-ipfs",
		TestCommit:         fmt.Sprintf("%x", sha1.Sum(b)),
		TestInstanceCount:  int(1 + (rand.Uint32() % 999)),
		TestInstanceRole:   "",
		TestInstanceParams: make(map[string]string, 0),
	}
}

// // NewContextWithRunEnv returns a new context containing the run environment.
// func NewContextWithRunEnv(ctx context.Context) context.Context {
// 	return context.WithValue(ctx, runEnvContextKey, CurrentRunEnv())
// }

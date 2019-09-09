package runtime

import (
	"context"
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

	runEnvContextKey key = iota
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	TestPlan    string `json:"test_plan"`
	TestCase    string `json:"test_case"`
	TestRun     string `json:"test_run"`
	TestCaseSeq int    `json:"test_seq"`

	TestRepo   string `json:"test_repo"`
	TestCommit string `json:"test_commit"`
	TestBranch string `json:"test_branch"`
	TestTag    string `json:"test_tag"`

	TestInstanceCount  int    `json:"test_instance_count"`
	TestInstanceRole   string `json:"test_instance_role"`
	TestInstanceParams string `json:"test_instance_params"`

	parsedParams map[string]string

	// TODO: we'll want different kinds of loggers.
	logger  *zap.Logger
	slogger *zap.SugaredLogger
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
	)
	re.slogger = re.logger.Sugar()
}

func (re *RunEnv) parseParams() {
	if re.parsedParams != nil {
		return
	}

	splt := strings.Split(re.TestInstanceParams, "|")
	m := make(map[string]string, len(splt))
	for _, s := range splt {
		v := strings.Split(s, "=")
		if len(v) != 2 {
			continue
		}
		m[v[0]] = v[1]
	}
	re.parsedParams = m
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	toInt := func(s string) (v int) {
		var err error
		if v, err = strconv.Atoi(s); err != nil {
			return -1
		}
		return v
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
		TestInstanceParams: os.Getenv(EnvTestInstanceParams),
	}

	re.initLoggers()
	re.parseParams()

	return re
}

// StringParam returns a string parameter.
func (re *RunEnv) StringParam(name string) (s string, ok bool) {
	v, ok := re.parsedParams[name]
	return v, ok
}

// IntParam returns an int parameter.
func (re *RunEnv) IntParam(name string) (i int, ok bool) {
	v, ok := re.parsedParams[name]
	if !ok {
		return -1, false
	}
	i, err := strconv.Atoi(v)
	return i, err == nil
}

// ExtractRunEnv extracts the test context from a context.Context object.
func ExtractRunEnv(ctx context.Context) *RunEnv {
	c := ctx.Value(runEnvContextKey)
	if c == nil {
		panic("test context is nil")
	}
	tctx, ok := c.(*RunEnv)
	if !ok {
		panic("test context has unexpected type")
	}
	return tctx
}

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
		TestInstanceCount:  int(rand.Uint32()),
		TestInstanceRole:   "",
		TestInstanceParams: "",
	}
}

// NewContextWithRunEnv returns a new context containing the run environment.
func NewContextWithRunEnv(ctx context.Context) context.Context {
	return context.WithValue(ctx, runEnvContextKey, CurrentRunEnv())
}

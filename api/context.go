package api

import (
	"context"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

const (
	EnvTestPlan    = "TEST_PLAN"
	EnvTestBranch  = "TEST_BRANCH"
	EnvTestCase    = "TEST_CASE"
	EnvTestTag     = "TEST_TAG"
	EnvTestRun     = "TEST_RUN"
	EnvTestRepo    = "TEST_REPO"
	EnvTestCaseSeq = "TEST_CASE_SEQ"
)

type key int

const (
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
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	toInt := func(s string) (v int) {
		var err error
		if v, err = strconv.Atoi(s); err != nil {
			// noop
		}
		return v
	}

	tc := &RunEnv{
		TestPlan:    os.Getenv(EnvTestPlan),
		TestCase:    os.Getenv(EnvTestCase),
		TestRun:     os.Getenv(EnvTestRun),
		TestTag:     os.Getenv(EnvTestTag),
		TestBranch:  os.Getenv(EnvTestBranch),
		TestRepo:    os.Getenv(EnvTestRepo),
		TestCaseSeq: toInt(os.Getenv(EnvTestCaseSeq)),
	}

	return tc
}

// RunEnvFromContext extracts the test context from a context.Context object.
func RunEnvFromContext(ctx context.Context) *RunEnv {
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

func RandomRunEnv() *RunEnv {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return &RunEnv{
		TestPlan:    fmt.Sprintf("testplan-%d", rand.Uint32()),
		TestCase:    fmt.Sprintf("testcase-%d", rand.Uint32()),
		TestRun:     fmt.Sprintf("testrun-%d", rand.Uint32()),
		TestCaseSeq: int(rand.Uint32()),
		TestRepo:    "github.com/ipfs/go-ipfs",
		TestCommit:  fmt.Sprintf("%x", sha1.Sum(b)),
	}
}

// NewContext returns a new context that carries the run environment.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, runEnvContextKey, CurrentRunEnv())
}

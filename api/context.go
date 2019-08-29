package api

import (
	"context"
	"os"
	"strconv"
)

type key int

const (
	runEnvContextKey key = iota
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	TestPlan    string `json:"test_plan"`
	TestCase    string `json:"test_case"`
	TestRun     int    `json:"test_run"`
	TestTag     string `json:"test_tag"`
	TestBranch  string `json:"test_branch"`
	TestRepo    string `json:"test_repo"`
	TestCaseSeq int    `json:"test_seq"`
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
		TestRun:     toInt(os.Getenv(EnvTestRun)),
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

// NewContext returns a new context that carries the run environment.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, runEnvContextKey, CurrentRunEnv())
}

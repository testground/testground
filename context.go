package testground

import "context"

var TestContextKey = "TestContext"

type TestContext struct {
	TestPlan string `json:"test_plan"`
	TestCase string `json:"test_case"`
	TestRun  int    `json:"test_run"`
}

func ExtractTestContext(ctx context.Context) *TestContext {
	c := ctx.Value(TestContextKey)
	if c == nil {
		panic("test context is nil")
	}
	tctx, ok := c.(*TestContext)
	if !ok {
		panic("test context has unexpected type")
	}
	return tctx
}

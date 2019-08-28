package testground

// WIP struct.
type TestPlanDescriptor struct {
	Name string
}

// WIP interface.
type TestPlan interface {
	TestCases() []TestCase
	Execute(seq int)
}

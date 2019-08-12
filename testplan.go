package tpipeline

type TestPlanDescriptor struct {
	Name string
}

type TestPlan interface {
	TestCases() []TestCase

	Execute(seq int)



}

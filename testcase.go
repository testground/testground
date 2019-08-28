package testground

type TestCaseDescriptor struct {
	Name string
}

type TestCase interface {
	// Descriptor returns the descriptor for this test case.
	Descriptor() *TestCaseDescriptor
}

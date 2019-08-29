package dht

import (
	"github.com/ipfs/testground/api"
)

type TestPlan struct{}

var _ api.TestPlan = (*TestPlan)(nil)

type testCase interface {
	Name() string
	Execute()
}

var testcases = []testCase{
	&lookupPeersTC{Count: 10, BucketSize: 100},
}

func (*TestPlan) Descriptor() *api.TestPlanDescriptor {
	names := make([]string, 0, len(testcases))
	for _, tc := range testcases {
		names = append(names, tc.Name())
	}

	desc := &api.TestPlanDescriptor{
		Name:      "dht-tests",
		TestCases: []string{"lookup-peers"},
	}
	return desc
}

func (*TestPlan) Build(opts *api.BuildOpts) (*api.BuildResult, error) {
	return nil, nil
}

func (*TestPlan) Schedule(*api.BuildResult, *api.ScheduleOpts) (*api.ScheduleResult, error) {
	return nil, nil
}

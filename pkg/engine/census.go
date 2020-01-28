package engine

import (
	"errors"
	"os"
	"sync"

	"github.com/ipfs/testground/pkg/api"
)

// TestCensus represents a test census. It is a singleton object managed by this
// package, and should not be instantiated explicitly, unless for testing
// purposes.
type TestCensus struct {
	lk sync.RWMutex
	m  map[string]*api.TestPlanDefinition
}

var _ api.TestCensus = (*TestCensus)(nil)

func newTestCensus() *TestCensus {
	return &TestCensus{
		m: make(map[string]*api.TestPlanDefinition),
	}
}

// EnrollTestPlan registers this test plan in the census.
func (c *TestCensus) EnrollTestPlan(tp *api.TestPlanDefinition) error {
	c.lk.Lock()
	defer c.lk.Unlock()

	if _, ok := c.m[tp.Name]; ok {
		return errors.New("test plan name is not unique")
	}

	tp.SourcePath = os.ExpandEnv(tp.SourcePath)
	c.m[tp.Name] = tp
	return nil
}

// ByName returns the test plan with the specified name, or nil if
// inexistent.
func (c *TestCensus) PlanByName(name string) *api.TestPlanDefinition {
	c.lk.RLock()
	defer c.lk.RUnlock()

	return c.m[name]
}

// List returns all test plans enrolled.
func (c *TestCensus) ListPlans() (tp []*api.TestPlanDefinition) {
	c.lk.RLock()
	defer c.lk.RUnlock()

	for _, e := range c.m {
		tp = append(tp, e)
	}
	return tp
}

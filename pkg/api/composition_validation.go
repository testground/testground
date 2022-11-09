package api

import (
	"fmt"
	"math"

	"github.com/go-playground/validator/v10"
)

var compositionValidator = func() *validator.Validate {
	v := validator.New()
	v.RegisterStructValidation(ValidateInstances, &Instances{})
	return v
}()

func (gs Groups) Validate(c *Composition) error {
	// Validate group IDs are unique
	m := make(map[string]struct{}, len(gs))
	for _, g := range gs {
		if _, ok := m[g.ID]; ok {
			return fmt.Errorf("group ids not unique; found duplicate: %s", g.ID)
		}
		m[g.ID] = struct{}{}
	}

	// Validate every group has a builder or there is a global
	for _, g := range gs {
		if g.Builder == "" && c.Global.Builder == "" {
			return fmt.Errorf("group %s is missing a builder", g.ID)
		}
	}

	return nil
}

// ValidateForBuild validates that this Composition is correct for a build.
func (c *Composition) ValidateForBuild() error {
	err := compositionValidator.StructExcept(c,
		"Global.Case",
		"Global.TotalInstances",
		"Global.Runner",
	)
	if err != nil {
		return err
	}

	return c.Groups.Validate(c)
}

// ValidateForRun validates that this Composition is correct for a run.
func (c *Composition) ValidateForRun() error {
	// Perform structural validation.
	if err := compositionValidator.Struct(c); err != nil {
		return err
	}

	// Calculate instances per group, and assert that sum total matches the
	// expected value.
	totalInstances := c.Global.TotalInstances

	computedTotal := uint(0)
	for _, g := range c.Groups {
		// When a percentage is specified, we require that totalInstances is set
		if g.Instances.Percentage > 0 && totalInstances == 0 {
			return fmt.Errorf("groups count percentage requires a total_instance configuration")
		}

		// Update the group's calculated instance counts.
		if g.calculatedInstanceCnt = g.Instances.Count; g.calculatedInstanceCnt == 0 {
			g.calculatedInstanceCnt = uint(math.Round(g.Instances.Percentage * float64(totalInstances)))
		}
		computedTotal += g.calculatedInstanceCnt
	}

	// Verify the sum total matches the expected value if it was passed.
	if totalInstances > 0 && totalInstances != computedTotal {
		// TODO: disabled temporarily
		return fmt.Errorf("sum of calculated instances per group doesn't match total; total=%d, calculated=%d", totalInstances, computedTotal)
	}

	c.Global.TotalInstances = computedTotal

	return c.Groups.Validate(c)
}

// ValidateInstances validates that either count or percentage is provided, but
// not both.
func ValidateInstances(sl validator.StructLevel) {
	instances := sl.Current().Interface().(Instances)

	if (instances.Count == 0 || instances.Percentage == 0) && (float64(instances.Count)+instances.Percentage > 0) {
		return
	}

	sl.ReportError(instances.Count, "count", "Count", "count_or_percentage", "")
	sl.ReportError(instances.Percentage, "percentage", "Percentage", "count_or_percentage", "")
}
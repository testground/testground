package api

import "io"

// Healthchecker is the interface to be implemented by a runner that can be
// healthcheckable.
type Healthchecker interface {
	Healthcheck(repair bool, writer io.Writer) (*HealthcheckReport, error)
}

type HealthcheckStatus string

type HealthcheckItem struct {
	Name    string
	Status  HealthcheckStatus
	Message string
}

type HealthcheckReport struct {
	Checks []HealthcheckItem
	Fixes  []HealthcheckItem
}

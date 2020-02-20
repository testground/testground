package api

import "io"

// Healthchecker is the interface to be implemented by a runner that supports
// healthchecks and repairs.
type Healthchecker interface {
	Healthcheck(repair bool, writer io.Writer) (*HealthcheckReport, error)
}

// HealthcheckStatus is an enum that represents
type HealthcheckStatus string

var (
	// HealthcheckStatusOK indicates success in a healthcheck or a repair.
	HealthcheckStatusOK = HealthcheckStatus("ok")
	// HealthcheckStatusFailed indicates the outcome of a healthcheck or an
	// attempted fix was negative.
	HealthcheckStatusFailed = HealthcheckStatus("failed")
	// HealthcheckStatusAborted indicates an internal error during the execution
	// of a healthcheck or a fix.
	HealthcheckStatusAborted = HealthcheckStatus("aborted")
	// HealthcheckStatusOmitted indicates that a healthcheck or a fix was not
	// carried out due to previous errors.
	HealthcheckStatusOmitted = HealthcheckStatus("omitted")
)

// HealthcheckItem represents an entry in a HealthcheckReport. It is used to
// convey the result of checks and fixes.
type HealthcheckItem struct {
	// Name is a short name describing this item.
	Name string
	// Status is the status of this check/fix.
	Status HealthcheckStatus
	// Message optionally contains any human-readable messages to be presented
	// to the user.
	Message string
}

type HealthcheckReport struct {
	// Checks enumerates the outcomes of the health checks.
	Checks []HealthcheckItem

	// Fixes enumerates the outcomes of the fixes applied during repair, if a
	// repair was requested.
	Fixes []HealthcheckItem
}

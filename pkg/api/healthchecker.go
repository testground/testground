package api

import (
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/rpc"
)

// Healthchecker is the interface to be implemented by a runner that supports
// healthchecks and fixes.
type Healthchecker interface {
	Healthcheck(fix bool, engine Engine, ow *rpc.OutputWriter) (*HealthcheckReport, error)
}

// HealthcheckStatus is an enum that represents
type HealthcheckStatus string

var (
	// HealthcheckStatusOK indicates success in a healthcheck or a fix.
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

	// Fixes enumerates the outcomes of the fixes applied during fix, if a
	// fix was requested.
	Fixes []HealthcheckItem
}

func (hr *HealthcheckReport) ChecksSucceeded() bool {
	for _, c := range hr.Checks {
		if c.Status != HealthcheckStatusOK && c.Status != HealthcheckStatusOmitted {
			return false
		}
	}
	return true
}

func (hr *HealthcheckReport) FixesSucceeded() bool {
	for _, f := range hr.Fixes {
		if f.Status != HealthcheckStatusOK && f.Status != HealthcheckStatusOmitted {
			return false
		}
	}
	return true
}

func (hr *HealthcheckReport) String() string {
	b := new(strings.Builder)

	if len(hr.Checks) > 0 {
		fmt.Fprintln(b, "Checks:")
		for _, check := range hr.Checks {
			fmt.Fprintf(b, "- %s: %s; %s\n", check.Name, check.Status, check.Message)
		}
	} else {
		fmt.Fprintln(b, "No checks made.")
	}

	if len(hr.Fixes) > 0 {
		fmt.Fprintln(b, "Fixes:")
		for _, fix := range hr.Fixes {
			fmt.Fprintf(b, "- %s: %s; %s\n", fix.Name, fix.Status, fix.Message)
		}
	} else {
		fmt.Fprintln(b, "No fixes applied.")
	}

	return b.String()
}

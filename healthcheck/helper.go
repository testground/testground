package healthcheck

import (
	"context"
	"fmt"
	"sync"

	"github.com/testground/testground/api"
)

// Checker is a function that checks whether a precondition is met. It returns
// whether the check succeeded, an optional message to present to the user, and
// error in case the check logic itself failed.
//
//   (true, *, nil) => HealthcheckStatusOK
//   (false, *, nil) => HealthcheckStatusFailed
//   (false, *, not-nil) => HealthcheckStatusAborted
//   checker doesn't run => HealthcheckStatusOmitted (e.g. dependent checks where the upstream failed)
type Checker func() (ok bool, msg string, err error)

// Fixer is a function that will be called to attempt to fix a failing check. It
// returns an optional message to present to the user, and error in case the fix
// failed.
type Fixer func() (msg string, err error)

type item struct {
	Name    string
	Checker Checker
	Fixer   Fixer
}

// Helper is a utility that facilitates the execution of healthchecks.
//
// Healthchecks are registered via the Enlist() method, which takes a name, a
// Checker, and a Fixer function. Common Checker and Fixer functions can be
// found in this package.
//
// To run the healthchecks and obtain an api.HealthcheckReport, call RunChecks.
//
// Healthchecks are run sequentially, in the same order they were listed.
//
// For each item, the Checker runs first. If it results in a "failed" status,
// and an associated Fixer is registered, we run the Fixer, if and only if
// "fix" mode is requested when calling RunChecks.
type Helper struct {
	sync.Mutex

	items  []*item
	report *api.HealthcheckReport
	err    error
}

// Enlist registers a new healthcheck, supplying its name, a compulsory Checker,
// and an optional Fixer.
func (h *Helper) Enlist(name string, c Checker, f Fixer) {
	h.Lock()
	defer h.Unlock()

	h.items = append(h.items, &item{name, c, f})
}

// RunChecks runs the checks and returns an api.HealthcheckReport, or a non-nil
// error if an internal error occured. See godocs on the Helper type for
// additional information.
func (h *Helper) RunChecks(_ context.Context, fix bool) (*api.HealthcheckReport, error) {
	h.Lock()
	defer h.Unlock()

	if h.report != nil {
		return h.report, h.err
	}

	h.report = new(api.HealthcheckReport)
	for _, li := range h.items {
		check := api.HealthcheckItem{Name: li.Name}

		// Check succeeds.
		ok, msg, err := li.Checker()
		switch {
		case err != nil:
			check.Status = api.HealthcheckStatusAborted
			check.Message = fmt.Sprintf("%s; error: %s", msg, err)
			h.report.Checks = append(h.report.Checks, check)

			if fix && li.Fixer != nil {
				h.report.Fixes = append(h.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
			}

		case ok:
			check.Status = api.HealthcheckStatusOK
			check.Message = msg
			h.report.Checks = append(h.report.Checks, check)

			if fix && li.Fixer != nil {
				h.report.Fixes = append(h.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusUnnecessary})
			}

		default:
			// Checker failed. We will attempt a fix action.
			check.Status = api.HealthcheckStatusFailed
			check.Message = msg
			h.report.Checks = append(h.report.Checks, check)

			if li.Fixer == nil {
				// no fixer; move on to next check.
				continue
			}

			if !fix {
				h.report.Fixes = append(h.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
				break
			}

			// Attempt fix if fix is enabled.
			// The fix might result in a failure, a successful recovery.
			var f api.HealthcheckItem
			msg, err := li.Fixer()
			if err != nil {
				f = api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusFailed, Message: msg}
			} else {
				f = api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOK, Message: msg}
			}

			h.report.Fixes = append(h.report.Fixes, f)
		}
	}

	return h.report, h.err
}

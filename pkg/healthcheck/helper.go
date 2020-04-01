package healthcheck

import (
	"context"

	"github.com/ipfs/testground/pkg/api"
)

type item struct {
	Name    string
	Checker Checker
	Fixer   Fixer
}

// HealthcheckHelper implements HealthcheckHelper. Runchecks runs each check and fix
// sequentially, in the order they are Enlist()'ed.
//
// HealthcheckHelper is a strategy interface for runners.
// Each runner may have required elements -- infrastructure, etc. which should be checked prior to
// running plans. Individual checks are registered to the HealthcheckHelper using the Enlist()
// method. With all of the checks enlisted, execute the checks, and optionally fixes, using the
// RunChecks() method. The details of how the checks are performed is implementation specific.
// Typically, the checker and fixer passed to the enlist method will be closures. These methods will
// be called when RunChecks is executed.
type HealthcheckHelper struct {
	items  []*item
	Report *api.HealthcheckReport
}

func (hh *HealthcheckHelper) Enlist(name string, c Checker, f Fixer) {
	hh.items = append(hh.items, &item{name, c, f})
}

func (hh *HealthcheckHelper) RunChecks(ctx context.Context, fix bool) error {
	for _, li := range hh.items {
		check := api.HealthcheckItem{Name: li.Name}

		// Check succeeds.
		ok, msg, err := li.Checker()
		switch {
		case err != nil:
			check.Status = api.HealthcheckStatusAborted
			check.Message = msg
			hh.Report.Checks = append(hh.Report.Checks, check)

			if fix {
				hh.Report.Fixes = append(hh.Report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
			}

		case ok:
			check.Status = api.HealthcheckStatusOK
			check.Message = msg
			hh.Report.Checks = append(hh.Report.Checks, check)

			if fix {
				hh.Report.Fixes = append(hh.Report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
			}

		default:
			// Checker failed. We will attempt a fix action.
			check.Status = api.HealthcheckStatusFailed
			check.Message = msg
			hh.Report.Checks = append(hh.Report.Checks, check)

			if !fix {
				hh.Report.Fixes = append(hh.Report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
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

			hh.Report.Fixes = append(hh.Report.Fixes, f)
		}
	}
	return nil
}

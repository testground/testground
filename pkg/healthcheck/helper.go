package healthcheck


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
	report *api.HealthcheckReport
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
			hh.report.Checks = append(hh.report.Checks, check)

			if fix {
				hh.report.Fixes = append(hh.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
			}

		case ok:
			check.Status = api.HealthcheckStatusOK
			check.Message = msg
			hh.report.Checks = append(hh.report.Checks, check)

			if fix {
				hh.report.Fixes = append(hh.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
			}

		default:
			// Checker failed. We will attempt a fix action.
			check.Status = api.HealthcheckStatusFailed
			check.Message = msg

			hh.report.Checks = append(hh.report.Checks, check)

			if !fix {
				hh.report.Fixes = append(hh.report.Fixes, api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOmitted})
				break
			}

			// Attempt fix if fix is enabled.
			// The fix might result in a failure, a successful recovery.
			var f api.HealthcheckItem
			msg, err := li.Fixer()
			if err != nil {
				f = api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusOK, Message: msg}
			} else {
				f = api.HealthcheckItem{Name: li.Name, Status: api.HealthcheckStatusFailed, Message: msg}
			}

			// Fill the report with fix information.
			hh.report.Checks = append(hh.report.Checks, check)
			hh.report.Fixes = append(hh.report.Fixes, f)
		}
	}
	return nil
}
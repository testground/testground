package runner

import (
	"context"
	"fmt"

	"github.com/ipfs/testground/pkg/api"
	"golang.org/x/sync/errgroup"
)

type Checker func() bool
type Fixer func() error

type Healthchecked interface {
	Enlist(name string, c Checker, f Fixer)
	RunChecks(ctx context.Context, fix bool)
}

type toDoElement struct {
	Name    string
	Checker Checker
	Fixer   Fixer
}

type ErrgroupHealthcheckHelper struct {
	toDo   []*toDoElement
	Report *api.HealthcheckReport
}

func (hh *ErrgroupHealthcheckHelper) Enlist(name string, c Checker, f Fixer) {
	hh.toDo = append(hh.toDo, &toDoElement{name, c, f})
}

func (hh *ErrgroupHealthcheckHelper) RunChecks(ctx context.Context, fix bool) {
	eg, _ := errgroup.WithContext(ctx)

	for _, li := range hh.toDo {
		hcp := *li
		eg.Go(func() error {
			// Checker succeeds, already working.
			if hcp.Checker() {
				hh.Report.Checks = append(hh.Report.Checks, api.HealthcheckItem{
					Name:    li.Name,
					Status:  api.HealthcheckStatusOK,
					Message: fmt.Sprintf("%s: OK", li.Name),
				})
				return nil
			}
			// Checker failed, Append the failure to the check report
			hh.Report.Checks = append(hh.Report.Checks, api.HealthcheckItem{
				Name:    li.Name,
				Status:  api.HealthcheckStatusFailed,
				Message: fmt.Sprintf("%s: FAILED. Fixing: %t", li.Name, fix),
			})
			// Attempt fix if fix is enabled.
			fixhc := api.HealthcheckItem{Name: li.Name}
			if fix {
				err := li.Fixer()
				if err != nil {
					// Oh no! the fix failed.
					fixhc.Status = api.HealthcheckStatusFailed
					fixhc.Message = fmt.Sprintf("%s FAILED: %v", li.Name, err)
				} else {
					// Fix succeeded.
					fixhc.Status = api.HealthcheckStatusOK
					fixhc.Message = fmt.Sprintf("%s RECOVERED", li.Name)
				}
				// Fill the report with fix information.
				hh.Report.Fixes = append(hh.Report.Fixes, fixhc)
				return nil
			}
			// don't attempt to fix.
			fixhc.Status = api.HealthcheckStatusOmitted
			fixhc.Message = fmt.Sprintf("%s recovery not attempted.", li.Name)
			return nil
		})
		eg.Wait()
	}
}

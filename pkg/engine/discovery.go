package engine

import (
	"fmt"
	"path/filepath"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
)

// discoverTestPlans scans the manifest directory for test plans, enrolls them
// all into the engine, and returns a slice of all hits.
func (e *Engine) discoverTestPlans() ([]*api.TestPlanDefinition, error) {
	glob := filepath.Join(e.envcfg.SrcDir, "/manifests/*.toml")
	manifests, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}

	defs := make([]*api.TestPlanDefinition, 0, len(manifests))
	for _, m := range manifests {
		def := new(api.TestPlanDefinition)
		if _, err := toml.DecodeFile(m, def); err != nil {
			logging.S().Errorf("failed to parse file %s: %s; skipping", m, err)
			continue
		}

		if err := e.census.EnrollTestPlan(def); err != nil {
			return nil, fmt.Errorf("failed to enroll discovered test plan: %w", err)
		}

		defs = append(defs, def)
	}

	return defs, nil
}

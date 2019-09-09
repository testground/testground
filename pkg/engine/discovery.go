package engine

import (
	"path/filepath"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"

	"github.com/BurntSushi/toml"
)

// discoverTestPlans returns the test plans in the manifests directory.
func discoverTestPlans() []*api.TestPlanDefinition {
	glob := filepath.Join(BaseDir, "/manifests/*.toml")
	manifests, err := filepath.Glob(glob)
	if err != nil {
		logging.S().Errorf("failed to glob manifests directory: %w", err)
		return nil
	}

	defs := make([]*api.TestPlanDefinition, 0, len(manifests))
	for _, m := range manifests {
		var def api.TestPlanDefinition
		if _, err := toml.DecodeFile(m, &def); err != nil {
			logging.S().Errorf("failed to parse file %s: %w", m, err)
			continue
		}
		defs = append(defs, &def)
	}
	return defs
}

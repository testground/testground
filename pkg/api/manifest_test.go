package api

import (
	"testing"

	"github.com/testground/testground/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestManifestHasBuilder(t *testing.T) {
	m := TestPlanManifest{
		Name: "manifest001",
		Builders: map[string]config.ConfigMap{
			"docker:go":      {},
			"docker:generic": {},
		},
	}

	require.True(t, m.HasBuilder("docker:go"))
	require.True(t, m.HasBuilder("docker:generic"))
	require.False(t, m.HasBuilder("docker:rust"))
	require.False(t, m.HasBuilder("anything"))
}

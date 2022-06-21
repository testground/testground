package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/api"
)

func TestBuildInputMayContainMultipleManifests(t *testing.T) {
	buildInput := &BuildInput{
		BuildRequest: &api.BuildRequest{
			Priority: 1,
			Composition: api.Composition{
				Metadata: api.Metadata{},
				Global:   api.Global{},
				Groups:   []*api.Group{},
			},
			Manifests: []api.TestPlanManifest{
				{
					Name: "test-plan-1",
				},
				{
					Name: "test-plan-2",
				},
			},
			CreatedBy: api.CreatedBy{},
		},
	}

	require.NotNil(t, buildInput)
	require.Len(t, buildInput.BuildRequest.Manifests, 2)
}

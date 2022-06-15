package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGroupsUnique(t *testing.T) {
	c := &Composition{
		Metadata: Metadata{},
		Global: Global{
			BuildableComposition: BuildableComposition{
				Plan:    "foo_plan",
				Case:    "foo_case",
				Builder: "docker:go",
			},
			Runner: "local:docker",
		},
		Groups: []*Group{
			{ID: "repeated"},
			{ID: "repeated"},
		},
	}

	require.Error(t, c.ValidateForBuild())
	require.Error(t, c.ValidateForRun())
}

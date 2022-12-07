//go:build integration && local_docker
// +build integration,local_docker

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	. "github.com/testground/testground/pkg/integration/utils"
)

func TestSmoke(t *testing.T) {
	Setup(t)
	require.Equal(t, 1, 1)
}

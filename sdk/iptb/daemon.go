package iptb

import (
	"context"

	ipfsClient "github.com/ipfs/go-ipfs-api"
)

// SpawnDaemon spawns a daemon using the InterPlanetary Test Bed and returns
// the ensemble (you must call ensemble.Destroy() in the end) and the client API
// connection.
func SpawnDaemon(ctx context.Context, opts NodeOpts) (*TestEnsemble, *ipfsClient.Shell) {
	spec := NewTestEnsembleSpec()
	spec.AddNodesDefaultConfig(opts, "node")

	ensemble := NewTestEnsemble(ctx, spec)
	ensemble.Initialize()

	node := ensemble.GetNode("node")
	client := node.Client()

	return ensemble, client
}

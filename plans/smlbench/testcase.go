package smlbench

import (
	"context"

	"github.com/ipfs/testground/api"
	"github.com/ipfs/testground/iptb"
)

type SmallBenchmarksTestCase interface {
	api.Namer

	// Configure configures the specification for the testcase.
	Configure(ctx context.Context, spec *iptb.TestEnsembleSpec)

	// Execute executes the test case with the given ensemble.
	Execute(ctx context.Context, ensemble *iptb.TestEnsemble)
}

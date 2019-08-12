package smlbench

import (
	"context"

	tpipeline "github.com/ipfs/test-pipeline"
	"github.com/ipfs/test-pipeline/iptb"
)

type SmallBenchmarksTestCase interface {
	tpipeline.TestCase

	// Configure configures the specification for the testcase.
	Configure(ctx context.Context, spec *iptb.TestEnsembleSpec)

	// Execute executes the test case with the given ensemble.
	Execute(ctx context.Context, ensemble *iptb.TestEnsemble)
}

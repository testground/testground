package smlbench

import (
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

type SmallBenchmarksTestCase interface {
	// Configure configures the specification for the testcase.
	Configure(runenv *runtime.RunEnv, spec *iptb.TestEnsembleSpec)

	// Execute executes the test case with the given ensemble.
	Execute(runenv *runtime.RunEnv, ensemble *iptb.TestEnsemble)
}

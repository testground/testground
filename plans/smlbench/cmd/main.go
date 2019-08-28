package main

import (
	"context"

	"github.com/ipfs/testground"
	"github.com/ipfs/testground/iptb"
	"github.com/ipfs/testground/plans/smlbench/cases"
)

// TODO:
//  Testcase abstraction.
//  Entrypoint demuxing (TEST_CASE_SEQ).
//  Pipe stdout to intercept messages.
//  Temporary directory from environment variable.
//  Error handling -- right now everything panics on failure.
func main() {
	for _, tc := range cases.TestCases {
		spec := iptb.NewTestEnsembleSpec()

		desc := tc.Descriptor()
		ctx := context.WithValue(context.Background(), testground.TestContextKey, &testground.TestContext{
			TestPlan: "small-benchmarks",
			TestCase: desc.Name,
			TestRun:  123,
		})

		tc.Configure(ctx, spec)

		ensemble := iptb.NewTestEnsemble(ctx, spec)
		ensemble.Initialize()

		tc.Execute(ctx, ensemble)

		ensemble.Destroy()
	}
}

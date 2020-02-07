package main

import (
	"context"
	"os"
	"strconv"

	test "github.com/ipfs/testground/plans/smlbench/test"
	utils "github.com/ipfs/testground/plans/smlbench/utils"
	iptb "github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// Inventory of Tests
var testCasesSet = [][]utils.SmallBenchmarksTestCase{
	{
		&test.SimpleAddTC{SizeBytes: 0},                // 0
		&test.SimpleAddTC{SizeBytes: 1024},             // 1kb
		&test.SimpleAddTC{SizeBytes: 64 * 1024},        // 64kb
		&test.SimpleAddTC{SizeBytes: 256 * 1024},       // 256kb
		&test.SimpleAddTC{SizeBytes: 512 * 1024},       // 512kb
		&test.SimpleAddTC{SizeBytes: 1024 * 1024},      // 1mb
		&test.SimpleAddTC{SizeBytes: 2 * 1024 * 1024},  // 2mb
		&test.SimpleAddTC{SizeBytes: 5 * 1024 * 1024},  // 5mb
		&test.SimpleAddTC{SizeBytes: 10 * 1024 * 1024}, // 10mb
	},
	{
		&test.SimpleAddGetTC{SizeBytes: 0},                // 0
		&test.SimpleAddGetTC{SizeBytes: 1024},             // 1kb
		&test.SimpleAddGetTC{SizeBytes: 64 * 1024},        // 64kb
		&test.SimpleAddGetTC{SizeBytes: 256 * 1024},       // 256kb
		&test.SimpleAddGetTC{SizeBytes: 512 * 1024},       // 512kb
		&test.SimpleAddGetTC{SizeBytes: 1024 * 1024},      // 1mb
		&test.SimpleAddGetTC{SizeBytes: 2 * 1024 * 1024},  // 2mb
		&test.SimpleAddGetTC{SizeBytes: 5 * 1024 * 1024},  // 5mb
		&test.SimpleAddGetTC{SizeBytes: 10 * 1024 * 1024}, // 10mb
	},
}

// TODO: Error handling -- right now everything panics on failure.
func main() {
	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	testCases := testCasesSet[runenv.TestCaseSeq]

	for i, tc := range testCases {
		_ = os.Setenv("TEST_CASE", tc.Name())
		_ = os.Setenv("TEST_CASE_SEQ", strconv.Itoa(i))

		ctx := context.Background()
		// ctx, _ := context.WithCancel(context.Background())

		spec := iptb.NewTestEnsembleSpec()
		tc.Configure(runenv, spec)

		ensemble := iptb.NewTestEnsemble(ctx, spec)
		ensemble.Destroy()

		err := tc.Execute(runenv, ensemble)
		if err != nil {
			panic(err)
		}

		ensemble.Initialize()
	}
}

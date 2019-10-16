package main

import (
	"fmt"
	// "os"
	// "strconv"

	test "github.com/ipfs/testground/plans/smlbench/test"
	utils "github.com/ipfs/testground/plans/smlbench/utils"
	// iptb "github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

/*
var testCases = []func(*runtime.RunEnv){
	test.SimpleAdd,
	test.SimpleAddGet,
}
*/

// Inventory of Tests
var testCasesSet = [][]utils.SmallBenchmarksTestCase{
	{
		&test.SimpleAddTC{},                 // 0
		&test.SimpleAddTC{1024},             // 1kb
		&test.SimpleAddTC{64 * 1024},        // 64kb
		&test.SimpleAddTC{256 * 1024},       // 256kb
		&test.SimpleAddTC{512 * 1024},       // 512kb
		&test.SimpleAddTC{1024 * 1024},      // 1mb
		&test.SimpleAddTC{2 * 1024 * 1024},  // 2mb
		&test.SimpleAddTC{5 * 1024 * 1024},  // 5mb
		&test.SimpleAddTC{10 * 1024 * 1024}, // 10mb
	},
	{
		&test.SimpleAddGetTC{},                 // 0
		&test.SimpleAddGetTC{1024},             // 1kb
		&test.SimpleAddGetTC{64 * 1024},        // 64kb
		&test.SimpleAddGetTC{256 * 1024},       // 256kb
		&test.SimpleAddGetTC{512 * 1024},       // 512kb
		&test.SimpleAddGetTC{1024 * 1024},      // 1mb
		&test.SimpleAddGetTC{2 * 1024 * 1024},  // 2mb
		&test.SimpleAddGetTC{5 * 1024 * 1024},  // 5mb
		&test.SimpleAddGetTC{10 * 1024 * 1024}, // 10mb
	},
}

// TODO:
//  Testcase abstraction.
//  Entrypoint demuxing (TEST_CASE_SEQ).
//  Pipe stdout to intercept messages.
//  Temporary directory from environment variable.
//  Error handling -- right now everything panics on failure.
func main() {
	// _ = os.Setenv("TEST_PLAN", "smlbenchmarks")
	// _ = os.Setenv("TEST_BRANCH", "master")
	// _ = os.Setenv("TEST_TAG", "")
	// _ = os.Setenv("TEST_RUN", uuid.New().String())

	runenv := runtime.CurrentRunEnv()
	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	testCases := testCasesSet[runenv.TestCaseSeq]

	for i, tc := range testCases {
		// _ = os.Setenv("TEST_CASE", tc.Name())
		// _ = os.Setenv("TEST_CASE_SEQ", strconv.Itoa(i))

		// ctx := api.NewContext(context.Background())

		fmt.Printf("Jim1 %v %v\n", i, tc)
		/*
		spec := iptb.NewTestEnsembleSpec()
		tc.Configure(runenv, spec)

		ensemble := iptb.NewTestEnsemble(ctx, spec)
		ensemble.Initialize()

		tc.Execute(runenv, ensemble)

		ensemble.Destroy()
		*/
	}
}

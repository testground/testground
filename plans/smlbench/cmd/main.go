package main

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

	// for i, tc := range cases.TestCases {
	// 	_ = os.Setenv("TEST_CASE", tc.Name())
	// 	_ = os.Setenv("TEST_CASE_SEQ", strconv.Itoa(i))

	// 	ctx := api.NewContext(context.Background())

	// 	spec := iptb.NewTestEnsembleSpec()
	// 	tc.Configure(ctx, spec)

	// 	ensemble := iptb.NewTestEnsemble(ctx, spec)
	// 	ensemble.Initialize()

	// 	tc.Execute(ctx, ensemble)

	// 	ensemble.Destroy()
	// }
}

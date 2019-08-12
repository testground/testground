package cases

import "github.com/ipfs/test-pipeline/plans/smlbench"

// TODO: API sugar for generating variations of test cases based on some function.
var TestCases = []smlbench.SmallBenchmarksTestCase{
	&simpleAddTC{},                 // 0
	&simpleAddTC{1024},             // 1kb
	&simpleAddTC{64 * 1024},        // 64kb
	&simpleAddTC{256 * 1024},       // 256kb
	&simpleAddTC{512 * 1024},       // 512kb
	&simpleAddTC{1024 * 1024},      // 1mb
	&simpleAddTC{2 * 1024 * 1024},  // 2mb
	&simpleAddTC{5 * 1024 * 1024},  // 5mb
	&simpleAddTC{10 * 1024 * 1024}, // 10mb

	&simpleAddGetTC{},                 // 0
	&simpleAddGetTC{1024},             // 1kb
	&simpleAddGetTC{64 * 1024},        // 64kb
	&simpleAddGetTC{256 * 1024},       // 256kb
	&simpleAddGetTC{512 * 1024},       // 512kb
	&simpleAddGetTC{1024 * 1024},      // 1mb
	&simpleAddGetTC{2 * 1024 * 1024},  // 2mb
	&simpleAddGetTC{5 * 1024 * 1024},  // 5mb
	&simpleAddGetTC{10 * 1024 * 1024}, // 10mb
}

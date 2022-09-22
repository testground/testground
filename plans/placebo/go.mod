module github.com/testground/testground/plans/placebo

go 1.16

// Note that this test contains a composition that
// will test the dependency rewrites. If you change
// the version here, you probably want to update
// the composition file.
require github.com/testground/sdk-go v0.3.1-0.20211012114808-49c90fa75405

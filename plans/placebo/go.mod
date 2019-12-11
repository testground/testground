module github.com/ipfs/testground/plans/placebo

go 1.13

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
	github.com/kr/pretty v0.1.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

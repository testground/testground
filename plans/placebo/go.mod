module github.com/ipfs/testground/plans/placebo

go 1.14

require (
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

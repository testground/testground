module github.com/ipfs/testround/plans/verify

go 1.14

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/ipfs/testground/sdk/sync v0.4.0
	github.com/sparrc/go-ping v0.0.0-20190613174326-4e5b6552494c // indirect
)

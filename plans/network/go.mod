module github.com/ipfs/testground/plans/network

go 1.14

require (
	github.com/ipfs/testground v0.4.0
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/ipfs/testground/sdk/sync v0.4.0
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

module github.com/ipfs/testround/plans/example

go 1.13

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/go-redis/redis/v7 v7.0.0-beta.5 // indirect
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p-core v0.3.0 // indirect
)

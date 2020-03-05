module github.com/ipfs/testround/plans/benchmarks

go 1.13

replace (
	github.com/ipfs/testground/sdk/network => ../../sdk/network
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/ipfs/go-cid v0.0.4 // indirect
	github.com/ipfs/testground/sdk/network v0.2.0
	github.com/ipfs/testground/sdk/runtime v0.2.0
	github.com/ipfs/testground/sdk/sync v0.2.0
	github.com/libp2p/go-libp2p-core v0.3.0 // indirect
)

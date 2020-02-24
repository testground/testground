module github.com/ipfs/testground/plans/lotus-debug

go 1.13

require (
	github.com/filecoin-project/lotus v0.2.8
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/multiformats/go-multiaddr-net v0.1.2
)

replace (
	github.com/filecoin-project/filecoin-ffi => ../lotus/extern/filecoin-ffi
	github.com/filecoin-project/lotus => ../lotus
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

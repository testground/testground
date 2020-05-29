module github.com/ipfs/testground/plans/lotus-js

go 1.13

require (
	github.com/filecoin-project/go-address v0.0.2-0.20200504173055-8b6f2fb2b3ef
	github.com/filecoin-project/go-jsonrpc v0.1.1-0.20200520183639-7c6ee2e066b4
	github.com/filecoin-project/lotus v0.3.0
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p-core v0.5.6
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/multiformats/go-multiaddr-net v0.1.5
	github.com/rs/cors v1.7.0
)

replace (
	github.com/filecoin-project/filecoin-ffi => ../lotus/extern/filecoin-ffi
	github.com/filecoin-project/lotus => ../lotus
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

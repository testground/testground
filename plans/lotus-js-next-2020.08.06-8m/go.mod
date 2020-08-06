module github.com/ipfs/testground/plans/lotus-js

go 1.13

require (
	github.com/filecoin-project/go-address v0.0.2-0.20200504173055-8b6f2fb2b3ef
	github.com/filecoin-project/go-fil-markets v0.5.3
	github.com/filecoin-project/go-jsonrpc v0.1.1-0.20200602181149-522144ab4e24
	github.com/filecoin-project/lotus v0.3.0
	github.com/filecoin-project/specs-actors v0.8.6
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/multiformats/go-multiaddr-net v0.1.5
	github.com/rs/cors v1.6.0
	github.com/supranational/blst v0.1.2-alpha.1
)

replace (
	github.com/filecoin-project/filecoin-ffi => ../lotus/extern/filecoin-ffi
	github.com/filecoin-project/lotus => ../lotus
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

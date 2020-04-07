module github.com/ipfs/testround/plans/benchmarks

go 1.14

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/ethereum/go-ethereum v1.9.12
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/ipfs/testground/sdk/runtime v0.3.0
	github.com/ipfs/testground/sdk/sync v0.3.0
	github.com/multiformats/go-multihash v0.0.13 // indirect
	github.com/prometheus/client_golang v1.4.1
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
)

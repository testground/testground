module github.com/ipfs/testround/plans/benchmarks

go 1.14

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/ipfs/testground/sdk/runtime v0.2.0
	github.com/ipfs/testground/sdk/sync v0.2.0
	github.com/multiformats/go-multihash v0.0.13 // indirect
	github.com/prometheus/client_golang v1.4.1
)

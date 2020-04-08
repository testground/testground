module github.com/ipfs/testround/plans/benchmarks

go 1.14

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/influxdata/influxdb-client-go v1.0.0
	github.com/ipfs/testground/sdk/runtime v0.3.0
	github.com/ipfs/testground/sdk/sync v0.3.0
	github.com/kubernetes/client-go v11.0.0+incompatible // indirect
	github.com/multiformats/go-multihash v0.0.13 // indirect
	github.com/prometheus/client_golang v1.4.1
)

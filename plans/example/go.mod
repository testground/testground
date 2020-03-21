module github.com/ipfs/testround/plans/example

go 1.14

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/ipfs/testground/sdk/runtime v0.3.0
	github.com/ipfs/testground/sdk/sync v0.3.0
	github.com/prometheus/client_golang v1.4.1
)

module github.com/ipfs/testround/plans/example

go 1.14

replace github.com/ipfs/testground/sdk => ../../sdk

require (
	github.com/ipfs/testground/sdk v0.4.0
	github.com/prometheus/client_golang v1.5.1
)

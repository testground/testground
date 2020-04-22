module github.com/ipfs/testround/plans/example

go 1.14

replace github.com/testground/testground/sdk => ../../sdk

require (
	github.com/testground/testground/sdk v0.4.0
	github.com/prometheus/client_golang v1.5.1
)

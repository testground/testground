module github.com/ipfs/testround/plans/example

go 1.13

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-openssl v0.0.4 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

module github.com/ipfs/testground/plans/smlbench

go 1.13

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	go.uber.org/zap v1.12.0 // indirect
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

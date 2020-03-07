module github.com/ipfs/testground/plans/smlbench

go 1.13

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.2.0
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

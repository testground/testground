module github.com/ipfs/testground/plans/chew-large-datasets

go 1.13

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/go-ipfs v0.4.22-0.20191108103059-ec748a7b5b2f
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/go-ipfs-files v0.0.4
	github.com/ipfs/go-unixfs v0.2.1
	github.com/ipfs/interface-go-ipfs-core v0.2.3
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

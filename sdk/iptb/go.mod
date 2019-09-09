module github.com/ipfs/testground/sdk/iptb

go 1.13

require (
	github.com/ipfs/go-ipfs-api v0.0.2
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/iptb v1.4.0
	github.com/ipfs/iptb-plugins v0.2.0
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
	github.com/multiformats/go-multiaddr v0.0.4
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

module github.com/ipfs/testground/sdk/iptb

go 1.14

require (
	github.com/ipfs/go-ipfs-api v0.0.3
	github.com/ipfs/go-ipfs-config v0.2.0
	github.com/ipfs/iptb v1.4.0
	github.com/ipfs/iptb-plugins v0.2.1
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/multiformats/go-multiaddr v0.2.0
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

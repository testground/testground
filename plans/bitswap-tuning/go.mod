module github.com/ipfs/testground/plans/bitswap-tuning

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

require (
	github.com/ipfs/go-bitswap v0.1.9
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.1
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipfs-chunker v0.0.3
	github.com/ipfs/go-ipfs-delay v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.6
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-merkledag v0.2.4
	github.com/ipfs/go-unixfs v0.2.2
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-openssl v0.0.4 // indirect
	github.com/multiformats/go-multihash v0.0.8
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.8.1
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

module github.com/testground/testground/plans/bitswap-tuning

go 1.14

replace github.com/testground/testground/sdk => ../../sdk

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
	github.com/testground/testground/sdk v0.4.0
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/multiformats/go-multihash v0.0.10
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
)

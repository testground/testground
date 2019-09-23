module github.com/ipfs/testground/plans/dht

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

require (
	github.com/ipfs/go-datastore v0.1.0
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/sync v0.0.0-00010101000000-000000000000
	github.com/libp2p/go-libp2p v0.3.1
	github.com/libp2p/go-libp2p-core v0.2.2
	github.com/libp2p/go-libp2p-kad-dht v0.2.1
)

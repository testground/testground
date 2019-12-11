module github.com/ipfs/testground/plans/dummy

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

require (
	github.com/ipfs/testground v0.0.0-20191210101804-ece200707681 // indirect
	github.com/ipfs/testground/plans/dht v0.0.0-20191210101804-ece200707681 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/libp2p/go-libp2p-kad-dht v0.3.1 // indirect
	github.com/urfave/cli v1.22.2 // indirect
)

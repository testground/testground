module github.com/ipfs/testground/plans/dht

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

require (
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.3.1
	github.com/ipfs/go-ipfs-util v0.0.1
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p v0.4.2
	github.com/libp2p/go-libp2p-connmgr v0.2.1
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-kad-dht v0.4.1
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	go.uber.org/zap v1.12.0
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

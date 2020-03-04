module github.com/ipfs/testground/plans/autonat

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/libp2p/go-libp2p-autonat => github.com/willscott/go-libp2p-autonat v0.1.2-0.20200303235034-f40dd4c74a3e

require (
	github.com/ipfs/go-cid v0.0.5
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p v0.5.3-0.20200227181042-85a83edf8055
	github.com/libp2p/go-libp2p-autonat v0.1.1
	github.com/libp2p/go-libp2p-autonat-svc v0.1.1-0.20200304022055-c1f9c7d0db8f
	github.com/libp2p/go-libp2p-circuit v0.1.4
	github.com/libp2p/go-libp2p-connmgr v0.2.1
	github.com/libp2p/go-libp2p-core v0.3.2-0.20200302164944-ee95739931de
	github.com/libp2p/go-libp2p-peerstore v0.1.4
	github.com/libp2p/go-libp2p-routing v0.1.0
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multiaddr-net v0.1.2
)

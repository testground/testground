module github.com/ipfs/testground/plans/secure-channel

go 1.13

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

require (
	github.com/ipfs/go-log v1.0.1 // indirect
	github.com/ipfs/go-log/v2 v2.0.2 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/ipfs/testground/sdk/sync v0.0.0-20190921111954-a84ff142a5a3
	github.com/libp2p/go-conn-security v0.1.0 // indirect
	github.com/libp2p/go-conn-security-multistream v0.1.0
	github.com/libp2p/go-libp2p v0.5.1
	github.com/libp2p/go-libp2p-blankhost v0.1.4
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-host v0.1.0 // indirect
	github.com/libp2p/go-libp2p-interface-connmgr v0.1.0 // indirect
	github.com/libp2p/go-libp2p-interface-pnet v0.1.0 // indirect
	github.com/libp2p/go-libp2p-metrics v0.1.0 // indirect
	github.com/libp2p/go-libp2p-mplex v0.2.1
	github.com/libp2p/go-libp2p-net v0.1.0 // indirect
	github.com/libp2p/go-libp2p-noise v0.0.0-20200120141346-1a9c5941b6c7
	github.com/libp2p/go-libp2p-peerstore v0.1.4
	github.com/libp2p/go-libp2p-protocol v0.1.0 // indirect
	github.com/libp2p/go-libp2p-secio v0.2.1
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/libp2p/go-libp2p-tls v0.1.2
	github.com/libp2p/go-libp2p-transport v0.1.0 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-stream-muxer v0.1.0 // indirect
	github.com/libp2p/go-stream-muxer-multistream v0.2.0
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/multiformats/go-varint v0.0.2 // indirect
	github.com/whyrusleeping/go-smux-multiplex v3.0.16+incompatible // indirect
	github.com/whyrusleeping/go-smux-multistream v2.0.2+incompatible // indirect
	github.com/whyrusleeping/go-smux-yamux v2.0.9+incompatible // indirect
	github.com/whyrusleeping/yamux v1.2.0 // indirect
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad // indirect
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa // indirect
	golang.org/x/sys v0.0.0-20200124204421-9fbb57f87de9 // indirect
	golang.org/x/tools v0.0.0-20200128002243-345141a36859 // indirect
)

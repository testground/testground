module github.com/testground/testground/plans/dht

go 1.14

replace github.com/testground/testground/sdk => ../../sdk

require (
	github.com/gogo/protobuf v1.3.1
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.4.1
	github.com/ipfs/go-ds-leveldb v0.4.1
	github.com/ipfs/go-ipfs-util v0.0.1
	github.com/ipfs/go-ipns v0.0.2
	github.com/testground/testground/sdk v0.4.0
	github.com/libp2p/go-libp2p v0.4.2
	github.com/libp2p/go-libp2p-connmgr v0.2.1
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-kad-dht v0.4.1
	github.com/libp2p/go-libp2p-kbucket v0.2.2
	github.com/libp2p/go-libp2p-swarm v0.2.3-0.20200210151353-6e99a7602774
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-libp2p-xor v0.0.0-20200330160054-7c8ff159b6e9
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/multiformats/go-multiaddr-net v0.1.2
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.14.1
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
)

//replace github.com/libp2p/go-libp2p-swarm => ../../../../libp2p/go-libp2p-swarm
//replace github.com/libp2p/go-libp2p-autonat => github.com/willscott/go-libp2p-autonat v0.1.2-0.20200310184838-ce79942134d7
//replace github.com/libp2p/go-libp2p-autonat-svc => github.com/libp2p/go-libp2p-autonat-svc v0.1.1-0.20200310185508-f21360000124
//replace github.com/libp2p/go-libp2p-kad-dht => ../../../../libp2p/go-libp2p-kad-dht
//replace github.com/libp2p/go-libp2p-kad-dht => github.com/libp2p/go-libp2p-kad-dht v0.5.2-0.20200310202241-7ada018b2a13
//replace github.com/libp2p/go-libp2p => github.com/libp2p/go-libp2p v0.6.1-0.20200310185355-89c193e0ca37
//replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.0

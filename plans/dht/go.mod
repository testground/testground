module github.com/ipfs/testground/plans/dht

go 1.13

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

replace github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

require (
	github.com/btcsuite/btcd v0.0.0-20190926002857-ba530c4abb35 // indirect
	github.com/ipfs/go-datastore v0.1.0
	github.com/ipfs/go-todocounter v0.0.2 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/ipfs/testground/sdk/sync v0.0.0-20190921111954-a84ff142a5a3
	github.com/libp2p/go-libp2p v0.4.0
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-kad-dht v0.2.1
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/multiformats/go-multiaddr-dns v0.1.1 // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	golang.org/x/crypto v0.0.0-20190926180335-cea2066c6411 // indirect
	golang.org/x/net v0.0.0-20190926025831-c00fd9afed17 // indirect
	golang.org/x/sys v0.0.0-20190927073244-c990c680b611 // indirect
)

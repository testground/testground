module github.com/ipfs/testground/sdk/sync

go 1.14

require (
	github.com/go-redis/redis/v7 v7.2.0
	github.com/ipfs/testground v0.4.0
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/prometheus/client_golang v1.4.1
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

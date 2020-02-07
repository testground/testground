module github.com/ipfs/testground/sdk/sync

go 1.13

require (
	github.com/go-redis/redis/v7 v7.0.0-beta.4
	github.com/hashicorp/go-multierror v1.0.0
	github.com/ipfs/testground v0.1.0
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/multiformats/go-multiaddr v0.1.1
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

module github.com/ipfs/testground/sdk/sync

go 1.14

require (
	github.com/go-redis/redis/v7 v7.2.0
	github.com/ipfs/testground v0.4.0
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/onsi/ginkgo v1.11.0 // indirect
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/prometheus/client_golang v1.4.1
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/net v0.0.0-20191109021931-daa7c04131f5 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

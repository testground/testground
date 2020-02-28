module github.com/ipfs/testground/plans/network

go 1.13

require (
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/ipfs/testground v0.1.0
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-openssl v0.0.4 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo v1.10.3 // indirect
	github.com/onsi/gomega v1.7.1 // indirect
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

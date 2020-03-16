module github.com/ipfs/testground/plans/network

go 1.13

require (
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/ipfs/testground v0.2.0
	github.com/ipfs/testground/sdk/runtime v0.2.0
	github.com/ipfs/testground/sdk/sync v0.2.0
	github.com/onsi/ginkgo v1.10.3 // indirect
	github.com/onsi/gomega v1.7.1 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

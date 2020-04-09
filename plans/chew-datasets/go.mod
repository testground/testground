module github.com/ipfs/testground/plans/chew-datasets

go 1.14

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/golang/mock v1.4.1 // indirect
	github.com/ipfs/go-ipfs v0.4.22-0.20191108103059-ec748a7b5b2f
	github.com/ipfs/go-ipfs-api v0.0.3
	github.com/ipfs/go-ipfs-config v0.2.0
	github.com/ipfs/go-ipfs-files v0.0.6
	github.com/ipfs/interface-go-ipfs-core v0.2.3
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9 // indirect
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

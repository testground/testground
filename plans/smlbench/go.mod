module github.com/ipfs/testground/plans/smlbench

go 1.13

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/libp2p/go-libp2p-core v0.2.2 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	go.opencensus.io v0.22.0 // indirect
	go.uber.org/zap v1.12.0 // indirect
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9 // indirect
	golang.org/x/sys v0.0.0-20190826190057-c7b8b68b1456 // indirect
	gopkg.in/yaml.v2 v2.2.4 // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

module github.com/ipfs/testground/plans/smlbench

go 1.13

require (
	github.com/aws/aws-sdk-go v1.28.10 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/testground v0.0.0-20200204192812-e3f72fb75c57 // indirect
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	k8s.io/api v0.17.2 // indirect
	k8s.io/client-go v11.0.0+incompatible // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

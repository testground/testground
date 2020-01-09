module github.com/ipfs/testground/plans/network

go 1.13

require (
	github.com/containernetworking/cni v0.7.1 // indirect
	github.com/ipfs/testground v0.0.0-20200111081546-c39eb09092da // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/ipfs/testground/sdk/sync v0.0.0-20190921111954-a84ff142a5a3
	k8s.io/client-go v11.0.0+incompatible // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime

replace github.com/ipfs/testground/sdk/sync => ../../sdk/sync

module github.com/ipfs/testground/plans/chew-datasets

go 1.13

require (
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/ipfs/go-ipfs v0.4.22-0.20191108103059-ec748a7b5b2f
	github.com/ipfs/go-ipfs-api v0.0.2
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/go-ipfs-files v0.0.6
	github.com/ipfs/interface-go-ipfs-core v0.2.3
	github.com/ipfs/testground v0.0.0-20200203172810-d076a151d33c // indirect
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ../../sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
)

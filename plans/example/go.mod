module github.com/ipfs/testround/plans/example

go 1.13

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)

require (
	github.com/go-redis/redis/v7 v7.0.0-beta.5 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/ipfs/go-ipfs v0.4.22 // indirect
	github.com/ipfs/go-ipfs-cmds v0.1.1 // indirect
	github.com/ipfs/go-ipfs-config v0.2.0 // indirect
	github.com/ipfs/go-log v1.0.1 // indirect
	github.com/ipfs/interface-go-ipfs-core v0.2.5 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20200127114353-d9d54ceef2b2
	github.com/ipfs/testground/sdk/sync v0.0.0-20190921111954-a84ff142a5a3
)

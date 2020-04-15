module github.com/ipfs/testground/sdk/sync

go 1.14

require (
	github.com/go-redis/redis/v7 v7.2.0
	github.com/ipfs/testground/sdk/runtime v0.4.0
	github.com/pkg/errors v0.9.1 // indirect
	go.uber.org/zap v1.12.0
	golang.org/x/net v0.0.0-20191109021931-daa7c04131f5 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/tools v0.0.0-20200227222343-706bc42d1f0d // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../runtime

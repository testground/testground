module github.com/ipfs/testground

go 1.12

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/go-redis/redis v6.15.5+incompatible
	github.com/google/uuid v1.1.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/nomad/api v0.0.0-20190902133951-9a44545dbe18
	github.com/ipfs/go-ds-leveldb v0.1.0
	github.com/ipfs/go-ipfs-api v0.0.2
	github.com/ipfs/go-ipfs-config v0.0.11
	github.com/ipfs/iptb v1.4.0
	github.com/ipfs/iptb-plugins v0.2.0
	github.com/libp2p/go-libp2p v0.3.1
	github.com/libp2p/go-libp2p-core v0.2.2
	github.com/libp2p/go-libp2p-kad-dht v0.2.0
	github.com/logrusorgru/aurora v0.0.0-20190803045625-94edacc10f9b
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/urfave/cli v1.20.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/text v0.3.2 // indirect
)

replace github.com/miekg/dns => github.com/miekg/dns v1.0.14

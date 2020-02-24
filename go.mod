module github.com/ipfs/testground

go 1.13

replace (
	github.com/ipfs/testground/sdk/iptb => ./sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ./sdk/runtime
	github.com/ipfs/testground/sdk/sync => ./sdk/sync
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/aws/aws-sdk-go v1.28.9
	github.com/containerd/containerd v1.3.2 // indirect
	github.com/containerd/continuity v0.0.0-20200107194136-26c1120b8d41 // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20200206084213-b5fc6ea92cde
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/go-playground/validator/v10 v10.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/go-getter v1.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.8
	github.com/ipfs/go-cid v0.0.4 // indirect
	github.com/ipfs/go-datastore v0.4.4 // indirect
	github.com/ipfs/go-log v1.0.2 // indirect
	github.com/ipfs/testground/plans/smlbench v0.0.0-20200221170422-4f93fa2782e2 // indirect
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/libp2p/go-libp2p-autonat v0.1.1 // indirect
	github.com/libp2p/go-libp2p-discovery v0.2.0 // indirect
	github.com/libp2p/go-libp2p-nat v0.0.5 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.1.4 // indirect
	github.com/libp2p/go-libp2p-secio v0.2.1 // indirect
	github.com/logrusorgru/aurora v0.0.0-20191017060258-dc85c304c434
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/multiformats/go-multistream v0.1.1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/otiai10/copy v1.0.2
	github.com/pborman/uuid v1.2.0
	github.com/urfave/cli v1.22.1
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	go.uber.org/zap v1.12.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200202164722-d101bd2416d5 // indirect
	golang.org/x/tools v0.0.0-20191216052735-49a3e744a425 // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190706005506-4ed54556a14a
)

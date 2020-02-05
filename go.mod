module github.com/ipfs/testground

go 1.13

replace (
	github.com/miekg/dns => github.com/miekg/dns v1.0.14

	// Fix builds on windows.
	golang.org/x/sys => golang.org/x/sys v0.0.0-20191128015809-6d18c012aee9
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/aws/aws-sdk-go v1.28.9
	github.com/containerd/containerd v1.3.2 // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20191219193418-de30f30e9054
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/go-playground/validator/v10 v10.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/go-getter v1.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.8
	github.com/ipfs/testground/sdk/runtime v0.1.0
	github.com/ipfs/testground/sdk/sync v0.1.0
	github.com/logrusorgru/aurora v0.0.0-20191017060258-dc85c304c434
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/otiai10/copy v1.0.2
	github.com/pborman/uuid v1.2.0
	github.com/urfave/cli v1.22.1
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	go.uber.org/zap v1.12.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190706005506-4ed54556a14a
)

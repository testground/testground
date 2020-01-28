module github.com/ipfs/testground

go 1.13

replace (
	github.com/ipfs/testground/sdk/iptb => ./sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ./sdk/runtime
	github.com/ipfs/testground/sdk/sync => ./sdk/sync
	github.com/miekg/dns => github.com/miekg/dns v1.0.14

	// Fix builds on windows.
	golang.org/x/sys v0.0.0-20190922100055-0a153f010e69 => golang.org/x/sys v0.0.0-20190920190810-ef0ce1748380
)

require (
	cloud.google.com/go v0.52.0 // indirect
	cloud.google.com/go/storage v1.5.0 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/aws/aws-sdk-go v1.28.9
	github.com/containerd/containerd v1.3.2 // indirect
	github.com/containerd/continuity v0.0.0-20200107194136-26c1120b8d41 // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/docker/docker v1.4.2-0.20191127125652-7c3d53ed640f
	github.com/go-redis/redis/v7 v7.0.0-beta.5 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-getter v1.4.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/imdario/mergo v0.3.8
	github.com/ipfs/go-cid v0.0.4 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20200127114353-d9d54ceef2b2
	github.com/ipfs/testground/sdk/sync v0.0.0-20200127114353-d9d54ceef2b2
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/libp2p/go-libp2p-core v0.3.0 // indirect
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/multiformats/go-varint v0.0.2 // indirect
	github.com/otiai10/copy v1.0.2
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/urfave/cli v1.22.2
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad // indirect
	golang.org/x/exp v0.0.0-20200119233911-0405dc783f0a // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200124204421-9fbb57f87de9 // indirect
	golang.org/x/tools v0.0.0-20200128002243-345141a36859 // indirect
	google.golang.org/genproto v0.0.0-20200127141224-2548664c049f // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
)

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
	github.com/BurntSushi/toml v0.3.1
	github.com/aws/aws-sdk-go v1.28.9
	github.com/containernetworking/cni v0.7.1
	github.com/davecgh/go-spew v1.1.1
	github.com/deixis/spine v0.1.1
	github.com/docker/docker v1.4.2-0.20191127125652-7c3d53ed640f
	github.com/go-playground/validator/v10 v10.1.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.0.0-20190624222214-25d8b0b66985 // indirect
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/go-getter v1.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.8
	github.com/ipfs/testground/sdk/iptb v0.0.0-00010101000000-000000000000 // indirect
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/ipfs/testground/sdk/sync v0.0.0-20190921111954-a84ff142a5a3
	github.com/logrusorgru/aurora v0.0.0-20191017060258-dc85c304c434
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/otiai10/copy v1.0.2
	github.com/pborman/uuid v1.2.0
	github.com/urfave/cli v1.22.1
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	go.uber.org/zap v1.12.0
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190706005506-4ed54556a14a
)

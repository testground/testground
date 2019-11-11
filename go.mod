module github.com/ipfs/testground

go 1.13

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/Microsoft/hcsshim v0.8.6 // indirect
	github.com/aws/aws-sdk-go v1.15.78
	github.com/containerd/containerd v1.2.9 // indirect
	github.com/containerd/continuity v0.0.0-20190827140505-75bee3e2ccb6 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20190910181529-415f8ecb65e8
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-getter v1.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.7
	github.com/ipfs/testground/sdk/runtime v0.0.0-20190921111954-a84ff142a5a3
	github.com/kr/pretty v0.1.0 // indirect
	github.com/logrusorgru/aurora v0.0.0-20190803045625-94edacc10f9b
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/otiai10/copy v1.0.1
	github.com/otiai10/curr v0.0.0-20190513014714-f5a3d24e5776 // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/urfave/cli v1.22.0
	go.opencensus.io v0.22.1 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/net v0.0.0-20190926025831-c00fd9afed17 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190927073244-c990c680b611 // indirect
	google.golang.org/grpc v1.23.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.4 // indirect
	gotest.tools v2.2.0+incompatible // indirect
)

replace (
	github.com/ipfs/testground/sdk/iptb => ./sdk/iptb
	github.com/ipfs/testground/sdk/runtime => ./sdk/runtime
	github.com/ipfs/testground/sdk/sync => ./sdk/sync
	github.com/miekg/dns => github.com/miekg/dns v1.0.14
)

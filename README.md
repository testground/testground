# InterPlanetary TestGround

![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg?style=flat-square)

> ⚠️ **Heavy WIP.** beware of the Dragons 🐉..

> **This repository is incubating the InterPlanetary Testground. 🐣**

## Description

You may have noticed a few test efforts with similar names underway! Testing at scale is a hard problem. We are indeed exploring and experimenting a lot, until we land on an end-to-end solution that works for us.

* Interplanetary Testbed (IPTB): https://github.com/ipfs/iptb
  * a simple utility to manage local clusters/aggregates of IPFS instances.
* libp2p testlab: https://github.com/libp2p/testlab
  * a Nomad deployer for libp2p nodes with a DSL for test scenarios.

The Interplanetary Test Ground aims to leverage the learnings and tooling resulting from those efforts to provide a scalable runtime environment for the execution of various types of tests and benchmarks, written in different languages, by different teams, targeting a specific commit of IPFS and/or libp2p, and quantifying its characteristics in terms of performance, resource and network utilisation, stability, interoperability, etc., when compared to other commits.

The Interplanetary Test Ground aims to be tightly integrated with the software engineering practices and tooling the IPFS and libp2p teams rely on.

## Architecture

Refer to the [specification](docs/SPEC.md) document.

## Repo Structure

```
.
├── README.md                       # This file
├── main.go                         # TestGround entrypoint file
├── cmd                             # TestGround CLI comamnds
│   ├── all.go
│   ├── build.go
│   ├── daemon.go
│   ├── enum_flag.go
│   ├── helpers.go
│   ├── list.go
│   ├── run.go
│   └── sidecar.go
├── manifests                       # Manifests for each test Plan. These exist independent from plans to enable plans to live elsewhere
│   ├── dht.toml
│   └── smlbench.toml
├── plans                           # The Test Plan. Includes Image to be run, Assertions and more
│   ├── dht
│   └── smlbench
├── sdk                             # SDK available to each test plan
│   ├── README.md
│   ├── iptb
│   ├── runtime
│   └── sync
├── docs                            # Documentation of the project
│   ├── SPEC.md
│   ├── container-hierarchy.png
│   ├── dashboard-mockup.png
│   ├── system-architecture.png
│   └── testcase-runtime-env.png
├── pkg                             # Internals to TestGround
│   ├── api
│   ├── auto
│   ├── build
│   ├── engine
│   ├── logging
│   ├── runner
│   ├── state
│   └── util
└── tools                           # ??
    └── src_generate.go
```

## Contributing & Testing

We kindly ask you to read through the SPEC first and give this project a run first in your local machine. It is a fast moving project at the moment and it might require some tinkering and experimentation to compesate the lack of documentation.

### Setup

Ensure that you are running go 1.13 or later (for gomod support)

```sh
> go version
go version go1.13.1 darwin/amd64
```

Download the repo and install the dependencies

```sh
> go get git@github.com:ipfs/testground.git
# ..fetch and install logs
> cd $GOPATH/src/github.com/ipfs/testground
```

Test that everything is installed correctly by running

```sh
» TESTGROUND_BASEDIR=`pwd` testground
resolved testground base dir from env variable: /Users/imp/code/go-projects/src/github.com/ipfs/testground
NAME:
   testground - A new cli application

   USAGE:
      testground [global options] command [command options] [arguments...]

   COMMANDS:
      run      (builds and) runs test case with name `testplan/testcase`
      list     list all test plans and test cases
      build    builds a test plan
      help, h  Shows a list of commands or help for one command

   GLOBAL OPTIONS:
      -v          verbose output (equivalent to INFO log level)
      --vv        super verbose output (equivalent to DEBUG log level)
     --help, -h  show help
```

### Running the tests locally

### Running the tests on the TestGround infrastructure

`To be Written once such infrastructure exists..`

### Run a single test outside of the orchestrator


## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

You can contact us on the freenode #ipfs-dev channel or attend one of our [weekly calls](https://github.com/ipfs/team-mgmt/issues/674).

## License

Dual-licensed: [MIT](./LICENSE-MIT), [Apache Software License v2](./LICENSE-APACHE), by way of the [Permissive License Stack](https://protocol.ai/blog/announcing-the-permissive-license-stack/).

# TestGround

![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg)
[![](https://travis-ci.com/ipfs/testground.svg?branch=master)](https://travis-ci.com/ipfs/testground)

> ⚠️ **Heavy WIP.** beware of the Dragons 🐉..

> **This repository is incubating the Testground. 🐣**

## Description

You may have noticed a few test efforts with similar names underway! Testing at scale is a hard problem. We are indeed exploring and experimenting a lot, until we land on an end-to-end solution that works for us.

-  Interplanetary Testbed (IPTB): https://github.com/ipfs/iptb
  - a simple utility to manage local clusters/aggregates of IPFS instances.
- libp2p testlab: https://github.com/libp2p/testlab
  - a Nomad deployer for libp2p nodes with a DSL for test scenarios.
- And others such as https://github.com/ipfs/interop and https://github.com/ipfs/benchmarks

The Test Ground aims to leverage the learnings and tooling resulting from those efforts to provide a scalable runtime environment for the execution of various types of tests and benchmarks, written in different languages, by different teams, targeting a specific commit of IPFS and/or libp2p, and quantifying its characteristics in terms of performance, resource and network utilisation, stability, interoperability, etc., when compared to other commits.

The Test Ground aims to be tightly integrated with the software engineering practices and tooling the IPFS and libp2p teams rely on.

## Team

The current TestGround Team is composed of:

- @raulk - Lead Architect, Engineer, Developer
- @daviddias - Engineer, Developer, acting as interim PM for the project
- @nonsense - Engineer, Developer, TestGround as a Service / Infrastructure Lead
- @jimpick - Engineer, Developer
- @stebalien - Engineer, Developer
- @hacdias - Engineer, Developer
- you! Yes, you can contribute as well, however, do understand that this is a brand new and fast moving project and so contributing might require extra time to onboard

To learn how this team works together read [HOW_WE_WORK](./docs/HOW_WE_WORK.md)

## How to use Test Ground

- Consult the [USAGE](./docs/USAGE.md) to learn how to get it running
- Refer to the [SPEC](docs/SPEC.md) document to understand how it all works.
- Consult the repo structure below to know where to find the multiple subsystems and test plans of TestGround

```bash
├── README.md                       # This file
├── docs                            # Documentation of the project
│   ├── SPEC.md
│   ├── ...
├── main.go                         # TestGround entrypoint file
├── cmd                             # TestGround CLI comamnds
│   ├── all.go
│   ├── ...
├── sdk                             # SDK available to each test plan
│   ├── runtime
│   └── ...
├── pkg                             # Internals to TestGround
│   ├── api
│   ├── ...
├── manifests                       # Manifests for each test Plan. These exist independent from plans to enable plans to live elsewhere
│   ├── dht.toml
│   └── smlbench.toml
├── plans                           # The Test Plan. Includes Image to be run, Assertions and more
│   ├── dht
│   └── smlbench
└── tools                           # ??
    └── src_generate.go
```

## Contributing

If you plan to contribute code, make sure to install the [pre-commit](https://github.com/pre-commit/pre-commit) tool, which manages our pre-commit hooks for things like linters, go fmt, go vet, etc.

We provide a `Makefile` rule to facilitate the setup:

```sh
$ make init
```

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

You can contact us on the freenode #ipfs-dev channel or attend one of our [weekly calls](https://github.com/ipfs/team-mgmt/issues/674).

## License

Dual-licensed: [MIT](./LICENSE-MIT), [Apache Software License v2](./LICENSE-APACHE), by way of the [Permissive License Stack](https://protocol.ai/blog/announcing-the-permissive-license-stack/).

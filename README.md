# Testground

![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg)
[![](https://travis-ci.com/ipfs/testground.svg?branch=master)](https://travis-ci.com/ipfs/testground)
[![Codecov](https://img.shields.io/codecov/c/gh/ipfs/testground)](https://codecov.io/gh/ipfs/testground)

> ⚠️ **Heavy WIP.** beware of the Dragons 🐉..

> **This repository is incubating the Testground. 🐣**


## What is Testground

Testground's goal is to provide a set of tools for testing next generation P2P applications (i.e. Filecoin, IPFS, libp2p & others).


## Table of Contents

- [Background](#background)
- [How to use Testground](#how-to-use-testground)
- [Team](#team)
- [Contributing](#contributing)
- [License](#license)


## Background

You may have noticed a few test efforts with similar names underway! Testing at scale is a hard problem. We are indeed exploring and experimenting a lot, until we land on an end-to-end solution that works for us.

-  Interplanetary Testbed (IPTB): https://github.com/ipfs/iptb
  - a simple utility to manage local clusters/aggregates of IPFS instances.
- libp2p testlab: https://github.com/libp2p/testlab
  - a Nomad deployer for libp2p nodes with a DSL for test scenarios.
- And others such as https://github.com/ipfs/interop and https://github.com/ipfs/benchmarks

Testground aims to leverage the learnings and tooling resulting from those efforts to provide a scalable runtime environment for the execution of various types of tests and benchmarks, written in different languages, by different teams, targeting a specific commit of IPFS and/or libp2p, and quantifying its characteristics in terms of performance, resource and network utilisation, stability, interoperability, etc., when compared to other commits.

Testground aims to be tightly integrated with the software engineering practices and tooling IPFS and libp2p teams rely on.


## How to use Testground

- Consult the [USAGE](./docs/USAGE.md) to learn how to get it running
- Refer to the [SPEC](docs/SPEC.md) document to understand how it all works.
- Consult the repo structure below to know where to find the multiple subsystems and test plans of Testground

```bash
├── README.md                       # This file
├── docs                            # Documentation of the project
│   ├── SPEC.md
│   ├── ...
├── main.go                         # Testground entrypoint file
├── cmd                             # Testground CLI commands
│   ├── all.go
│   ├── ...
├── sdk                             # SDK available to each test plan
│   ├── runtime
│   └── ...
├── pkg                             # Internals to Testground
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


## Team

The current Testground Team is composed of:

- @raulk - Lead Architect, Engineer, Developer
- @daviddias - Engineer, Developer, acting as interim PM for the project
- @nonsense - Engineer, Developer, Testground as a Service / Infrastructure Lead
- @jimpick - Engineer, Developer
- @stebalien - Engineer, Developer
- @hacdias - Engineer, Developer
- you! Yes, you can contribute as well, however, do understand that this is a brand new and fast moving project and so contributing might require extra time to onboard

To learn how this team works together read [HOW_WE_WORK](./docs/HOW_WE_WORK.md)


## Contributing

Please read our [CONTRIBUTING Guidelines](./CONTRIBUTING.md) before making a contribution.


## License

Dual-licensed: [MIT](./LICENSE-MIT), [Apache Software License v2](./LICENSE-APACHE), by way of the [Permissive License Stack](https://protocol.ai/blog/announcing-the-permissive-license-stack/).

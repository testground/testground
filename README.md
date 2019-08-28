# interplanetary test ground

⚠️ **Heavy WIP.** ⚠️

> **This repository is incubating the Interplanetary Testground. 🐣**

You may have noticed a few test efforts with similar names underway! Testing at
scale is a hard problem. We are indeed exploring and experimenting a lot, until
we land on an end-to-end solution that works for us. 

* Interplanetary Testbed (IPTB): https://github.com/ipfs/iptb
  * a simple utility to manage local clusters/aggregates of IPFS instances.
* libp2p testlab: https://github.com/libp2p/testlab
  * a Nomad deployer for libp2p nodes with a DSL for test scenarios.

The Interplanetary Test Ground aims to leverage the learnings and tooling
resulting from those efforts to provide a scalable runtime environment for the
execution of various types of tests and benchmarks, written in different
languages, by different teams, targeting a specific commit of IPFS and/or
libp2p, and quantifying its characteristics in terms of performance, resource
and network utilisation, stability, interoperability, etc., when compared to
other commits.

The Interplanteary Test Ground aims to be tightly integrated with the software
engineering practices and tooling the IPFS and libp2p teams rely on.

## System architecture

Refer to the [specification](docs/SPEC.md) document.

## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

This repository falls under the IPFS [Code of
Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

You can contact us on the freenode #ipfs-dev channel or attend one of our
[weekly calls](https://github.com/ipfs/team-mgmt/issues/674).

## License

Dual-licensed: [MIT](./LICENSE-MIT), [Apache Software License
v2](./LICENSE-APACHE), by way of the [Permissive License
Stack](https://protocol.ai/blog/announcing-the-permissive-license-stack/).

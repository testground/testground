# Context

> Testground is a platform for testing, benchmarking, and simulating distributed and peer-to-peer systems at scale.
> It's designed to be multi-lingual and runtime-agnostic, scaling gracefully from 2 to 10k instances when needed.

Testground was used successfully at Protocol Labs

- TODO: note on libp2p interop
- TODO: note on filecooin
- TODO: note on IPFS v0.5.0 (massive DHT and Bitswap improvements)
- TODO: note on libp2p (gossipsub 1.1 security extensions)

# Vision

Testground As A Service,

- Related Products
    - https://cockroachdb.github.io/pebble/?max=local
    - https://perf.webkit.org/v3/


# Problems we focus on

## Table of Content

  * [1. Testground provides reliable results](#1-testground-provides-reliable-results)
  * [2. Testground can be set up as an organization-wide Service](#2-testground-can-be-set-up-as-an-organization-wide-service)
  * [3. The knowledge required to work efficiently with Testground is available and easy to access](#3-the-knowledge-required-to-work-efficiently-with-testground-is-available-and-easy-to-access)
  * [4. Testground Developement follows high-standard](#4-testground-developement-follows-high-standard)
  * [5. Testground provides the networking tooling required to test complex Distributed / Decentralized Applications](#5-testground-provides-the-networking-tooling-required-to-test-complex-distributed---decentralized-applications)
  * [6. Testground provides the tooling to make identifying and fixing issues easy.](#6-testground-provides-the-tooling-to-make-identifying-and-fixing-issues-easy)
  * [7. Testground covers every /essential/ combinations of languages and libraries required by its users](#7-testground-covers-every--essential--combinations-of-languages-and-libraries-required-by-its-users)


## 1. Testground provides reliable results

**Why:** A testing platform that is unreliable is just a noise-producing machine. We need to secure our users' trust. Testground maintainers need clear feebdacks about stability improvements & regressions.

### Milestone 1: We have a stability metrics dashboard

Maintainers and users have a way to measure and follow Testground's Stability over any "relevant" axis.

This might combine different languages, runners (k8s, docker, local), and context (developer env, CI env, k8s env).


### Milestone 2: Every known stability issue has been solved

- local runner,
- docker runner,
- EKS runner,


### Milestone 3: We have a performances dashboard

Maintainers and users have a way to measure and follow Testground's Performances over any "relevant" axis.


## 2. Testground can be set up as an organization-wide Service

**Why:** We believe Testground can provide value accross entire organizations. Making it easy to run large-scale workload and efficient small-scale CI tests is core to it's success.

### Milestone 1: We can use Testground EKS clusters in production

- Deploy
- Security
- Stability


### Milestone 2: We can measure and improve Testground's EKS stability
- Deploying short-lived clusters for benchmarking


### Milestone 3: We can measure and improve Testground's EKS performances
- Deploying short-lived clusters for benchmarking


### Milestone 4: Dashboard & CI Toolings

## 3. The knowledge required to work efficiently with Testground is available and easy to access

**Why:** We believe test is valuable when everyone in the team can contribute to testing. The platform has to be approachable.

Related - [EPICS 1741](https://github.com/testground/testground/issues/1471).


### Milestone 1: There is a working quickstart that is the entry point to every other documentation


### Milestone 2: Testground has an official release process

- We distribute versions following an explicit release process (no more `:edge` by default)
- It's easy to follow changes (CHANGELOG)
- We distribute binaries


### Milestone 3: Testground provides extensive documentation

- The documentation is up-to-date
    - Generates configuration documentation
- We provide introduction guides for every language
- We provide doc for Common Patterns


### Milestone 4: Potential users know about Testground's existence and features

- "Public Relations".


## 4. Testground Developement follows high-standard

**Why:** Testground has already provided value amongst many different projects. As it grows it will expand in different directions and it's critical that we can support these with efficient developement processes and project management. Being an open source project, external contribution is critical.

### Testground CI & Testing is practical and reliable

(I feel like this should be a top-level point)

- We use a single CI (GitHub CI)
- Refactor the testing to be easier to use and simpler to maintain
    - remove shell scripts
- Plan for testing EKS
- Measure & remove flakiness


### I have all the documentation I need as a Maintainer

- public specifications
- public discussions
- community gatherings


### Efficient Project Management and Project Visibility

- We have a single maintainer team for review,
- We have a clear label and triaging process,
- We have a clear contribution process (protected branches, etc),
- We have precise project management tooling (re-ignite ZenHub?)


## 5. Testground provides the networking tooling required to test complex Distributed / Decentralized Applications

**Why:** This is a main selling point for Testground.

### Milestone 1: ExplicitNetwork Simulation Support

There is a clear matrix of what features can be used with which Testground runner.

- MTU issues, networking simulation, etc.
- This will be a Matrix of feature x runner x precision (transport level, application level, etc.).


### Milestone 2: Testground provides support for important networks topologies
- Access to public networks - [issue 1472](https://github.com/testground/testground/issues/1472)
- NAT simulation - [issue 1299](https://github.com/testground/testground/issues/1299)
- Complex topologies - [issue 1354](https://github.com/testground/testground/issues/1354)


### Milestone 3: Testground provides a way to run tests over real networks
- Remote runners feature


## 6. Testground provides the tooling to make maintaining tests, identifying issues, and fixing issue simple.

**Why:**

- composition files specification and improvements -
- Logging improvements - [Epic 1355](https://github.com/testground/testground/issues/1355)
- tcpdump'ing features - [Issue #1384](https://github.com/testground/testground/issues/1384)


## 7. Testground covers every /essential/ combinations of languages and libraries required by its users

**Why:** Drive adoptions outside of Protocol Labs.

### We support the main language needed by our users

- Provide a simple matrix about which languages and builders are supported and how
    - example: go support = great, nodejs support = deprecated, python support = non existent
- Provide a way to raise requests for more language support
- Provide an example  + SDK for "most" languages using `docker:generic`
    - `rust`,  `browser`, `nim`, `python`,
- Provide an official `docker:xxxx` builder for "most" languages (`docker:rust`, `docker:browser`)
    - This will require deep knowledge of the packaging systems and how to do library rewrites, etc. See the `lipb2p/test-plans` rust support for this.


### We support the tooling needed to run interop tests in CI
- Custom setup & composition generation scripts


### We Support SDK Implementers

- Record every known SDK and its level of maintenance in an official "awesome-testground" page
    - example: nim-sdk.
- Provide instructions and a path for SDK implementers


# Later

## Testground is used at ProtocolLabs to measure performances

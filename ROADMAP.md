# Testground Roadmap

## Context

> Testground is a platform for testing, benchmarking, and simulating distributed and peer-to-peer systems at scale.
> It's designed to be multi-lingual and runtime-agnostic, scaling gracefully from 2 to 10k instances when needed.

Testground was used successfully at Protocol Labs

- TODO: note on libp2p interop
- TODO: note on filecoin
- TODO: note on IPFS v0.5.0 (massive DHT and Bitswap improvements)
- TODO: note on libp2p (gossipsub 1.1 security extensions)

## Vision

Testground As A Service embodies our long-term vision.

- A single, scalable platform that one or more organizations can use,
- The ability to track the impact of a change in terms of stability & performance across multiple projects,
- The ability to experiment with large-scale networks and simplify the integration testing of libraries used across deep stacks.

Products with similar ideas, but specialized in different areas:

- database: [CockroachDB performance tracker](https://cockroachdb.github.io/pebble/?max=local),
- browser: [Webkit Performance Dashboard](https://perf.webkit.org/v3/)

## Problems we focus on

We focus on the following:

1. Reliability: above all, Testground should be trusted by its users,
2. Usefulness: solving needs that have been requested explicitly by projects,
3. Sustainability: implementing the processes & tools we need to maintain Testground in the medium and long term.

We want to ensure Testground is valuable and stable before we grow its feature set.

### Table of Content

- [1. Testground provides reliable results](#1-testground-provides-reliable-results)
- [2. Testground can be set up as an organization-wide Service](#2-testground-can-be-set-up-as-an-organization-wide-service)
- [3. The knowledge required to work efficiently with Testground is available and easy to access](#3-the-knowledge-required-to-work-efficiently-with-testground-is-available-and-easy-to-access)
- [4. Testground Development follows high-standard](#4-testground-development-follows-high-standard)
- [5. Testground provides the networking tooling required to test complex Distributed / Decentralized Applications](#5-testground-provides-the-networking-tooling-required-to-test-complex-distributed--decentralized-applications)
- [6. Testground provides the tooling to make test maintenance & finding issues simple](#6-testground-provides-the-tooling-to-make-test-maintenance--finding-issues-simple)
- [7. Testground covers every essential combination of languages and libraries required by its users](#7-testground-covers-every-essential-combination-of-languages-and-libraries-required-by-its-users)

### 1. Testground provides reliable results

**Why:** an unreliable testing platform is just a noise machine. We need to secure our users' trust. Testground maintainers need clear feedback about stability improvements & regressions.

- We expect strictly zero false positives (a test succeeds because Testground missed an error); these are critical bugs we already test for.
- However, Testground users might encounter false negatives (a test fails because Testground encountered an issue). Our stability metrics will measure this.

#### Milestone 1: We have a stability metrics dashboard

Maintainers and users have a way to measure and follow Testground's Stability over any "relevant" axis.

This might combine different languages, runners (k8s, docker, local), and context (developer env, CI env, k8s env).

This dashboard will describe, explicitly, what to expect in term of false negatives (when an error is caused by testground itself and not by the plan or the test).

#### Milestone 2: We have identified and reached our user's stability requirements. We resolved "most" of them

- local runner,
- docker runner,
- EKS runner,

#### Milestone 3: We have a performances dashboard

Maintainers and users have a way to measure and follow Testground's Performances over any "relevant" axis. This effort will start with identifying which metrics we want to measure first.

It might contain:

- Raw build / run performance for synthetic workload
- Performance of real-life usage in CI (like `libp2p/test-plans`)
- Consideration for caching features, re-building, etc.

### 2. Testground can be set up as an organization-wide Service

**Why:** We believe Testground can provide value across entire organizations. Making it easy to run a large-scale workload and efficient small-scale CI tests are core to its success.

#### Milestone 1: We can use Testground EKS clusters in production

- Deploy
- Security
- Stability

#### Milestone 2: We can measure and improve Testground's EKS stability

- Deploying short-lived clusters for benchmarking

#### Milestone 3: We can measure and improve Testground's EKS performance

- Deploying short-lived clusters for benchmarking

#### Milestone 4: Dashboard & CI Tooling

### 3. The knowledge required to work efficiently with Testground is available and easy to access

**Why:** We believe testing is valuable when everyone on the team can contribute. Our platform has to be approachable.

Related - [EPICS 1741](https://github.com/testground/testground/issues/1471).

#### Milestone 1: There is a working quickstart that is the entry point to every other documentation

#### Milestone 2: Testground has an official release process

- We distribute versions following an explicit release process (no more `:edge` by default)
- It's easy to follow changes (CHANGELOG)
- We distribute binaries

#### Milestone 3: Testground provides extensive documentation

- The documentation is up-to-date
  - Generates configuration documentation
- We provide introduction guides for every language
- We provide doc for Common Patterns

#### Milestone 4: Potential users know about Testground's existence and features

- "Public Relations".

### 4. Testground Development follows high-standard

**Why:** Testground has already provided value amongst many different projects. Now, we need bulletproof development processes to make the project sustainable and facilitate external contributions.

#### Testground CI & Testing is practical and reliable

(I feel like this should be a top-level point)

- We use a single CI (GitHub CI)
- Refactor the testing to be easier to use and simpler to maintain
  - remove shell scripts
- Plan for testing EKS
- Measure & remove flakiness

#### I have all the documentation I need as a Maintainer

- public specifications
- public discussions
- community gatherings

#### Efficient Project Management and Project Visibility

- We have a single maintainer team for review,
- We have a clear label and triaging process,
- We have a reliable & transparent contribution process (protected branches, etc),
- We have precise project management tooling (port previous ZenHub planning into Github?)

### 5. Testground provides the networking tooling required to test complex Distributed / Decentralized Applications

**Why:** This is Testground's main feature.

#### Milestone 1: Explicit Network Simulation Support Documentation

There is a clear matrix of what features can be used with which Testground runner.

- MTU issues, networking simulation, etc.
- This will be a Matrix of feature x runner x precision (transport level, application level, etc.).

#### Milestone 2: Testground provides support for important network topologies

- Access to public networks - [issue 1472](https://github.com/testground/testground/issues/1472)
- NAT simulation - [issue 1299](https://github.com/testground/testground/issues/1299)
- Complex topologies - [issue 1354](https://github.com/testground/testground/issues/1354)

#### Milestone 3: Testground provides a way to run tests over real networks

- Remote runners feature
  - See [Notion](https://www.notion.so/pl-strflt/Remote-Runners-c4ad4886c4294fb6a6f8afd9c0c5b73c) design,
  - And [PR 1425](https://github.com/testground/testground/pull/1425) preliminary work.

### 6. Testground provides the tooling to make test maintenance & finding issues simple

**Why:**

- composition files specification and improvements -
- Logging improvements - [Epic 1355](https://github.com/testground/testground/issues/1355)
- tcpdump'ing features - [Issue #1384](https://github.com/testground/testground/issues/1384)

### 7. Testground covers every essential combination of languages and libraries required by its users

**Why:** Drive adoption outside of Protocol Labs.

#### We support the primary languages needed by our users

- Provide a simple matrix about which languages and builders are supported and how
  - example: go support = great, nodejs support = deprecated, python support = non existent
- Provide a way to raise requests for more language support
- Provide an example  + SDK for "most" languages using `docker:generic`
  - `rust`,  `browser`, `nim`, `python`,
- Provide an official `docker:xxxx` builder for "most" languages (`docker:rust`, `docker:browser`)
  - This will require deep knowledge of the packaging systems and how to do library rewrites, etc. See the `lipb2p/test-plans` rust support for this.

#### We support the tooling needed to run interop tests in CI

- Custom setup & composition generation scripts

#### We Support SDK Implementers

- Record every known SDK and its level of maintenance in an official "awesome-testground" page
  - example: nim-sdk.
- Provide instructions and a path for SDK implementers

## Later

### Testground is used at ProtocolLabs to measure performances

### Testground security is top-notch

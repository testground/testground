# Testground Roadmap

```
Date: 2022-10-07
Status: In Progress
Notes: This document is still in review and may be heavily modified based on stakeholder feedback. Please add any feedback or questions in:
https://github.com/testground/testground/issues/1491
```

## Table Of Content

- [Context](#context)
  - [About the Roadmap](#about-the-roadmap)
- [Vision](#vision)
- [Our Focus for 2022-2023](#our-focus-for-2022-2023)
- [Milestones](#milestones)
  - [1. Bootstrap libp2p's interoperability testing story](#1-bootstrap-libp2ps-interoperability-testing-story)
  - [2. Refresh Testground's EKS support](#2-refresh-testgrounds-eks-support)
  - [3. Improve our Development & Testing infrastructure to meet engineering standards](#3-improve-our-development--testing-infrastructure-to-meet-engineering-standards)
  - [4. Provide a Testground As A Service Cluster used by libp2p & ipfs teams](#4-provide-a-testground-as-a-service-cluster-used-by-libp2p--ipfs-teams)
  - [5. Testground Is Usable by Non-Testground experts](#5-testground-is-usable-by-non-testground-experts)
  - [6. Support libp2p's interoperability testing story and ProbeLabs work as a way to drive "critical" Testground improvements](#6-support-libp2ps-interoperability-testing-story-and-probelabs-work-as-a-way-to-drive-critical-testground-improvements)
- [Appendix: Problems we focus on](#appendix-problems-we-focus-on)

## Context

> Testground is a platform for testing, benchmarking, and simulating distributed and peer-to-peer systems at scale.
> It's designed to be multi-lingual and runtime-agnostic, scaling gracefully from 2 to 10k instances when needed.

Testground was used successfully at Protocol Labs to validate improvements like libp2p's gossipsub security extensions,
IPFS' massive DHT and Bitswap improvement, and Filecoin improvements. Today, we are actively working on it to support libp2p interoperability testing.

### About the Roadmap

This document consists of two sections:

- [Milestones](#milestones) is the list of key improvements we intend to release with dates and deliverables. It is the "deliverable" oriented slice of our plan.
- [Appendix: Problems we focus on](#appendix-problems-we-focus-on) is the list of areas of improvement we intend to focus on. Some contain a few examples of milestones. It is the "problem" oriented slice of our plan and might cover multiple releases and multiple "milestones".

There is an issue dedicated to [Tracking Discussions around this Roadmap](https://github.com/testground/testground/issues/1491).

The timeline we share is our best-educated guess (not a hard commitment) around when we plan to provide critical improvements.

Where possible, we've shared a list of deliverables for every Milestone. These lists are not exhaustive or definitive; they aim to create a sense of "what are concrete outcomes" delivered by an improvement.

As we agree on this Roadmap and refine it, we'll create & organize missing EPICs.

Making Testground project management sustainable is [one of our key milestones](#3-improve-our-development--testing-infrastructure-to-meet-engineering-standards).

## Vision

_This section is still a very high-level draft_

Testground As A Service embodies our long-term vision.

- A single, scalable, installation that one or more organizations can use in all their projects,
- The ability to experiment with large-scale networks and simplify the integration testing of "any" network-related code,
- The ability to track the impact of a change in terms of stability & performance across multiple projects,
  - An example: having the ability to run IPFS benchmarks and simulations with different combinations of libraries. This would help us measure regression and improvements as soon as they occur in upstream dependencies.

Products with similar ideas but specialized in different areas:

- database: [CockroachDB performance tracker](https://cockroachdb.github.io/pebble/?max=local),
- browser: [Webkit Performance Dashboard](https://perf.webkit.org/v3/)

Researchs and Templates

- [Mockup for the Testground Dashboard](https://github.com/testground/testground/issues/133#issuecomment-553538029)

## Our Focus for 2022-2023

We focus on the following:

1. Reliability: above all, Testground should be trusted by its users,
2. Sustainability: implementing the processes & tools we need to maintain Testground in the medium and long term.
3. Usefulness: solving needs that have been requested explicitly by projects,

We want to ensure Testground is valuable and stable before we grow its feature set.

## Milestones

### 1. Bootstrap libp2p's interoperability testing story

- Delivery: Q4 2022
- Theme: usefulness
- Effort: approx. 6 months

**Why:** Testground provides an excellent foundation for distributed testing. Supporting the libp2p team with interoperability tests generates long-term value outside of Testground. We can move faster, generate interest, and create measurable improvements by focusing on a use-case.

**Deliverables:**

- Tooling to support interoperability testing (mixed-builders, composition templating),
- Stability measurements & fixes to reach the libp2p team's expectations,
- A fully working example (ping test) used in go-libp2p and rust-libp2p CIs,
- An interoperability Dashboard that shows how implementations and versions are tested.

### 2. Refresh Testground's EKS support

- Delivery: Q4 2022
- Theme: usefulness
- Effort: approx. 6 months

**Why:** Testground can simulate small networks in CI, but it covers more use cases when it lives in a larger cluster. When we run Testgroun in Kubernetes, we can support whole organizations through the Testground As A Service product.

Using a managed service (Amazon's Elastic Kubernetes Service) means our maintenance costs are lower, and the team can focus on improvements.

**Deliverables:**

- An EKS installation script,
  - Extra care is taken on Network infrastructure (CNIs).
- A (fixed) Kubernetes runner that runs on Amazon's EKS,
- The team can use the latest Testground version, create new releases, and upgrade the cluster.

### 3. Improve our Development & Testing infrastructure to meet engineering standards

- Delivery: Q1 2023
- Theme: reliability & sustainability
- Effort: approx. 4 months

**Why:** Testground proved itself valuable multiple times over the years. However, now we need bulletproof development processes to make the project sustainable and facilitate external contributions.

Extra care is taken on Testing and Stability: we are building a testing platform, and Testground's testing must be impeccable.

**Deliverables:**

- Automated release process with official releases, binary distribution, and Changelogs - [EPIC 1430](https://github.com/testground/testground/issues/1430)
- Well-defined Project management processes like triaging, contribution & reviewing processes, etc.
- Documentations for maintainers,
- Maintainable integration testing tooling (no more shell scripts or flakiness),
- A Stability Dashboard used to identify regression & discuss improvement with Maintainers and Users,
- Tooling for EKS testing.

### 4. Provide a Testground As A Service Cluster used by libp2p & ipfs teams

- Delivery: Q3 2023
- Theme: usefulness
- Effort: approx. 4 months

**Why:** Improving experience with faster testing in user's CI. Cover more use cases with large-scale networks.

**Deliverables:**

- A _stable_ cluster,
- Authentication,
- Tooling for users to use EKS cluster in their testing,
- Integration of the EKS feature in our testing infrastructure
  - Short-lived clusters used during integration testing

### 5. Testground Is Usable by Non-Testground experts

- Delivery: Q4 2023
- Theme: sustainability
- Effort: approx. 8 months

**Why:** Testground is an Open Source project, and the more project that uses it, the more improvement we'll see. We want to drive adoption outside of Protocol Labs but also encourage contribution. Other organizations have picked up the tool and started contributing; we want to lower friction and strike the Iron while it's hot.

**Deliverables:**

- Working Examples (tested in CI)
- Documentation [EPICS 1741](https://github.com/testground/testground/issues/1471).
  - Updated documentation infrastructure
  - Quickstart guides
  - Updated Examples & Tested in CI
  - New features & parameters, etc.
  - guides for most helpful use cases and features
  - composition templating, etc.
- Usability improvements
- SDK implementers support
  - Matrix of supported languages with links to SDKs
  - Instructions for SDK Implementers

### 6. Support libp2p's interoperability testing story and ProbeLabs work as a way to drive "critical" Testground improvements

- Delivery: Q4 2023
- Theme: usefulness
- Effort: approx. 8 months

**Why:** By focusing on a use case, we can move faster, generate interest, and create measurable improvements outside the project.

**Deliverables:**

- Javascript & Browser support in Testground - [issue 1386](https://github.com/testground/testground/issues/1386)
- Logging improvements - [Epic 1355](https://github.com/testground/testground/issues/1355)
- Reliable Network simulation in Docker and EKS
  - Access to public networks - [issue 1472](https://github.com/testground/testground/issues/1472)
  - NAT simulation - [issue 1299](https://github.com/testground/testground/issues/1299)
  - Complex topologies - [issue 1354](https://github.com/testground/testground/issues/1354)
  - Network Simulation Fixes - [Epic 1492](https://github.com/testground/testground/issues/1492)
- Remote-Runners for transport Benchmarking
  - See [Notion](https://www.notion.so/pl-strflt/Remote-Runners-c4ad4886c4294fb6a6f8afd9c0c5b73c) design,
  - And [PR 1425](https://github.com/testground/testground/pull/1425) preliminary work.
- Performance benchmarking tooling
- Debug tooling
  - tcpdump-related features - [Issue #1384](https://github.com/testground/testground/issues/1384)
- Composition Improvements

## Appendix: Problems we focus on

### 1. Testground provides reliable results

**Why:** an unreliable testing platform is just a noise machine. We need to secure our users' trust. In addition, testground maintainers need clear feedback about stability improvements & regressions.

- We expect strictly zero false positives (a test succeeds because Testground missed an error); these are critical bugs we already test for.
- However, Testground users might encounter false negatives (a test fails because Testground encountered an issue). Our stability metrics will measure this.

#### Milestone 1: We have a stability metrics dashboard

Maintainers and users have a way to measure and follow Testground's Stability over any "relevant" axis.

This might combine different languages, runners (k8s, docker, local), and contexts (developer env, CI env, k8s env).

This dashboard will explicitly describe what to expect regarding false negatives (when an error is caused by Testground itself and not by the plan or the test).

#### Milestone 2: We have identified and reached our user's stability requirements. We resolved "most" of them

- local runner,
- docker runner,
- EKS runner,

#### Milestone 3: We have a performances dashboard

Maintainers and users have a way to measure and follow Testground's Performances over any "relevant" axis. Therefore, this effort will start with identifying which metrics we want to measure first.

It might contain:

- Raw build / run performance for synthetic workload
- Performance of real-life usage in CI (like `libp2p/test-plans`)
- Consideration for caching features, re-building, etc.

### 2. Testground can be set up as an organization-wide Service

**Why:** We believe Testground can provide value across entire organizations. Making it easy to run a large-scale workload and efficient small-scale CI tests are core to its success.

#### Milestone 1: Users are able to start an EKS cluster and use it

- Deploy

#### Milestone 2: There is a Testground As A Service product that orgs like libp2p and ipfs can use

- Security (authentication)
- domain setup

#### Milestone 3: We can measure and improve Testground's EKS stability

- Deploying short-lived clusters for benchmarking

#### Milestone 4: We can measure and improve Testground's EKS performance

- Deploying short-lived clusters for benchmarking

#### Milestone 5: Dashboard & CI Tooling

### 3. The knowledge required to work efficiently with Testground is available and easy to access

**Why:** We believe testing is valuable when everyone on the team can contribute. Our platform has to be approachable.

Related - [EPICS 1741](https://github.com/testground/testground/issues/1471).

#### Milestone 1: There is a working quickstart that is the entry point to every other documentation

#### Milestone 2: Testground provides extensive documentation

- The documentation is up-to-date
  - Generates configuration documentation
- We provide introduction guides for every language
- We provide doc for Common Patterns

#### Milestone 3: Potential users know about Testground's existence and features

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

#### Testground has an official release process

- We distribute versions following an explicit release process (no more `:edge` by default)
- It's easy to follow changes (CHANGELOG)
- We distribute binaries
- The release process is automated

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

### 7. Testground covers every essential combination of languages, runtimes, and libraries required by its users

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

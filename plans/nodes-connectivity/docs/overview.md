# Overview

The "Nodes Connectivity" test plan will test the following:

> Ensuring that a node can always connect to the rest of the network, if not completely isolated.

The following software features will be tested:

* Transports
* Hole Punching
* Relay

# Primary Deliverables

* multiple networking scenarios
* reuseable Testground components that can be used by libp2p and IPFS developers building new connectivity features
* documentation, tutorials and visual explainers

# Challenges

1. There are a multitude of network scenarios and combinations of software to test

  We will need to develop many tests, starting with simple scenarios, but tackling
  progressively more complex scenarios over time.

  We can start with simple libp2p "ping" style tests for core connectivity, but
  we also want to test "real world" software such as go-ipfs and js-ipfs
  (Node.js and Browser) and how they interoperate in different network
  topologies.

  Additionally, we want to be able to apply workloads that stress the networking
  software so we can accurately characterize utilization, saturation and generate 
  error situations.

2. Networking is mostly invisible

  Ideally, we want P2P networking to be highly resilient and robust in the presence
  of challenges. Short of complete connectivity failure, bugs may just manifest
  themselves as poor performance. We will need to use every trick in the book
  to instrument the networking so that we can directly observe what is going on.
  We will need to pioneer techniques using tools such as Kibana and potentially
  other visualization tools to be able to understand complex networking systems.

3. We will use some "exotic" networking and observability features

  There is a wealth of available software that can be used to construct the test
  environments, but they tend to require serious study in order to use properly
  and it can be difficult to disseminate the knowledge widely amongst the
  test developers.

4. "Open-ended" scope

  There are an unlimited number of connectivity scenarios that can be imagined.
  Due to limited resources, we will need to be strategic and concentrate on
  the scenarios that deliver the highest impact and "return on investment".
  Decisions will need to be made about how many resources to devote to this
  test plan and over what time scale.

# Development Process

This test plan lives within the [ipfs/testground](https://github.com/ipfs/testground) repo. See [HOW_WE_WORK](https://github.com/ipfs/testground/blob/master/docs/HOW_WE_WORK.md) for the high level process.

A subset of contributors will be contributing to this test plan, which will have
a small amount of necessary test plan specific planning in addition to the top
level Testground planning.

The test plan will be developed iteratively over time, targeting a series of
"[milestones](milestones.md)", which will be decided using GitHub pull requests
and reviews.

Potentially, because of it's open-ended nature, the backlog of ideas and tasks
for this test plan could be quite huge. In order to keep the top-level Testground
[Zenhub board](https://app.zenhub.com/workspaces/test-ground-5db6a5bf7ca61c00014e2961/board?repos=197244214) tidy, we'll want to put some constraints in place to limit the
number of issues and PRs in the "Backlog" and "Icebox" state. Ideally issues are
granular enough that they can be completed in a couple of days or less. 6 to 10
issues on the ZenHub board is probably enough to keep some forward visibility
without cluttering the backlog too much. For tracking longer lists of things
to do, we can populate this "docs" directory (testplan-specific) with Markdown
files, or use Markdown checkbox lists in the comments on [Epic Issue #96](https://github.com/ipfs/testground/issues/96).

We want to have enough tasks identified at all times so that design and development work can be accurately tracked and so that contributors and stakeholders can easily comment on
and help guide development before work proceeds.

TODO - Talk about:

* design blueprints
* relationship to other test suites (eg. libp2p/interop, ipfs/interop)
* benchmarking
* PLAN_SPEC document
* related projects
* incubating components of Testground
* how to investigate possible technologies to incorporate
* reporting results and findings
* use of small "capstone" apps for real-world scenarios
* how to measure impact
* reviews: what? who? how?
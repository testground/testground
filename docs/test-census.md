# Available Test Suites accross {go, js}-IPFS and libp2p land

Complete, in-progress, and TBD.

## Sharness

* Status: ~50% Complete
* Language: Bash
* Type: Integration/API
* Target: Js & Go have separate suites. Work has been done to unify them but that was never completed.
* Where: CI

## Go CoreAPI Tests

* Status: ~25% Complete
* Language: Go
* Type: Integration/API
* Target: Go (for now), Js (future)
* Where: CI

## Js CoreAPI Tests

* Status: ~86% Complete
* Language: JS
* Type: Integration/API
* Target: JS & Go
* Where: CI

## Go Integration Benchmarks

* Status: They work but we don't run them automatically.
* Language: Go
* Type: Benchmark
* Target: Go Only
* Where: Local (now), Testing Infra (future)

## Libp2p Interop Tests

* Status: Active (Vasco). Run manually?
* Language: JS
* Type: Integration/API
* Target: JS & Go
* Where: Local? (has .travis.yml, but doesn’t seem to be active)

## Kubernetes-IPFS

* Status: Mothballed? Last active 2018
* Language: YAML-based DSL ([example](https://github.com/ipfs/kubernetes-ipfs/blob/master/tests/moving-medium.yml)), implemented with Go
* Type: Integration / Benchmark?
* Target: Go
* Where: Local (Minikube), Kubernetes

## Filecoin Automation & System Toolkit (FAST) + localnet

* Status: Active
* Language: Go
* Type: Integration / Benchmark?
* Target: Go
* Where: Local, CI?

## PeerPad / peer-base Tests

* Status: Active
* Language: JS
* Type: Integration / Load (OpenCensus/Jaeger support on branch)
* Target: JS
* Where: Local
* Want: WebRTC / NAT testing, Puppeteer / WebDriver + UI screen recordings, large CRDT scalability, replace *-star with rendezvous, offline testing

## Package Manager Benchmarks

* Status: Not started yet
* Language: Go
* Type: Integration / Benchmark
* Target: Go
* Where: Local, Testing Infra
* Want: quick content discovery, large file ingests and transfers, IPNS - pubsub, pinning, multiwriter

## IPFS Cluster Testing

* Status: Not started (it was Kubernetes-ipfs)
* Language: Go
* Type: -
* Want: IPTB/Kubernetes-IPFS like tests (end to end, on a cluster swarm, using the Cluster API to perform operations) that gather key metrics and traces. It should be quite aligned with how a swarm of ipfs daemons is tested.

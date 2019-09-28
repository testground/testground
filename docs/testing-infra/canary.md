# Production canary tests

**Status:** WIP
**Owner:** Test Infra

## What is tested

* How a change improves/deterioriates things when run against the live network?
* Indirectly measure network health.
* How changes to nightly affect performance on the network.

## How is it tested

Run...

* master
* previous releases
* nightly
* specific PRs

...against the real network; several iterations per test run to acquire multiple
observations and thus mitigate variance.

## Inputs

Benchmark scenarios.

## Outputs

* [ ] pprof profiles (go)
* [ ] Bandwidth usage
* [ ] allocation/goroutine totals (go)
* [ ] Flamegraph (js)
* [ ] Time till $event (time series)
* [ ] Packet rate.
* [ ] Custom, context-sensitive metrics per test case.

## Targets

* [ ] js-ipfs
* [ ] go-ipfs
* [ ] go-libp2p-daemon
* [ ] js-libp2p-daemon

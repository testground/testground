# Canary

**Status:** WIP
**Owner:** Test Infra

## What is tested

* Network Health
* How changes to nightly affect performance on the network.

## How is it tested

Run...

* master
* previous releases
* nightly
* specific PRs

...against the real network.

## Inputs

Benchmark scenarios.

## Outputs

* [ ] Pprof profiles (go)
* [ ] Bandwidth usage
* [ ] allocation/goroutine totals (go)
* [ ] Flamegraph (js)
* [ ] Time till $event (time series)
* [ ] Packet rate.

## Targets

* [ ] js-ipfs
* [ ] go-ipfs
* [ ] go-libp2p-daemon
* [ ] js-libp2p-daemon

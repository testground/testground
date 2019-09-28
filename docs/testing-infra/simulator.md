# Network Simulator

**Status:** WIP
**Owner:** Test Infra

## What is tested

How protocol changes affect the network when deployed at scale.

## How is it tested

* Spin up 10k-100k nodes
* Ship a set of IPFS/libp2p nodes (possibly running different versions)
* Run a complicated test scenario
* On demand.

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

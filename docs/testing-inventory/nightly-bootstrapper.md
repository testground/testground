# Nightly Bootstrapper

**Status:** WIP

**Owner:** ?

## What you want tested

master acting as a bootstrapper in the live network, refreshed every night, with a lifetime
of 48h.

## How is it tested

Two machines `A` and `B` are assigned to this test strategy. We flip between `A` and `B` on
every run in a round robin fashion, such that each gets redeployed in alternate nights.

Every night, go-ipfs/master gets built, we shutdown go-ipfs in the target, and redeploy the
new version.

## Inputs

Nothing.

## Outputs

Diagnostics traces:

* [ ] pprof profiles -- captured periodically and dispatched to a diagnostics service.
* [ ] Crash dumps -- captured on crash, and before restart.

OpenCensus metrics:

* [ ] Connection latency (time from TCP SYN till the peer shows up in the peer list)
* [ ] Peer counts/dial rates
* [ ] Bandwidth usage
* [ ] Packet rates

## Targets

* [ ] go-ipfs

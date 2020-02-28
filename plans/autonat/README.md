# `Plan:` The go-libp2p AutoNAT behaves

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

Nodes understanding of their connectivity should be reasonable.

## What is under test?

- The correctness and efficiency of the network detection functions in LibP2P

## Plan parameters

- ** Network Parameters **
  - `instances` - Number of nodes spun up for interconnectivity

## Tests

### Test: `Public Nodes`

This test spins up a core set of reachable 'bootstrap' nodes, and a second set of unreachable 'nat' nodes.

- ** Narrative **
  - ** Setup **
    - Nodes boot up
    - Nodes all are given a full routing table (IPs of each other node)
  - ** Act 1 **
    - Nodes attempt dialing each other.
    - Nodes expect to receive their expected address from the bootstrap nodes.
    - State of nodes local understanding of their network state is expected correct at the end.

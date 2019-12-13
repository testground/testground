# Testground testing Plan Specification

Each testing **Plan** contains:
- A **status badge**
  - ![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square) - The spec and/or the implementation of the Test Plan is very much raw or rapidly changing.
  - ![](https://img.shields.io/badge/status-stable-green.svg?style=flat-square) - We consider this test plan to close to final, it might be improved but it should not change fundamentally.
  - ![](https://img.shields.io/badge/status-reliable-brightgreen.svg?style=flat-square) - This Test Plan has been fully implemented and it is currently used in the Test Suite of a project. The output of the Test Plans should be taken in with attention.
  - ![](https://img.shields.io/badge/status-deprecated-red.svg?style=flat-square) - This test plan is no longer in use.
- An **overview** of what we are looking to achieve with the test (roughly ~1 paragraph).
- **What we are looking to expect to be able to optimize** by running this test and therefore, a suggestion of what are The data points that must be gathered in order to assess if an improvement or regression has been made.-
- The **plan parameters**. This include both Network Parameters (e.g. Number of Nodes) and Image Parameters (e.g. bucket_size, bitswap strategy, etc)
- The Tests. Each contains
  - A set of **test parameters** that are customizable for each test.
  - A **narrative** for each Test that describes on how the network will set up itself (_Warm Up_ phase) and how the actors will play their multiple roles (in _Acts_).

## Plan Template

```
# `Plan:` ???

## What is being optimized (min/max, reach)

- Metric 1
- Metric 2
- ...

## Plan Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
- **Image Parameters**
  - b

## Tests

### `Test:` _NAME_

- **Test Parameters**
  - n/a
- **Narrative**
  - **Warm up**
    - a
  - **Act I**
    - b
  - **Act II**
    - c
  - **Act III**
    - d
```

## Testing Plans MVP

The testing Plans linked below have been identified as the most valuable sets of tests to implement for the Testground MVP. The goal of delivering a good characterization of the performance of IPFS in specific areas that can bolster our confidence level in shipping high quality releases of go-ipfs. Once the first 10 test Plans are written and Testground fully deployed and operation, we will continue expanding the types of tests available.

- 01. [`Plan:` Chewing strategies for DataSets](../plans/chew-datasets)
- 02. [`Plan:` Data Transfer (Bitswap/GraphSync)](../plans/data-transfer)
- 03. [`Plan:` Nodes Connectivity (Transports, Hole Punching, Relay)](../plans/nodes-connectivity)
- 04. [`Plan:` Message Delivery (PubSub)](../plans/message-delivery)
- 05. [`Plan:` Naming (IPNS & its multiple routers)](../plans/naming)
- 06. [`Plan:` Bitswap specifics](../plans/bitswap-tuning)
- 07. [`Plan:` go-libp2p DHT beaves](../plans/dht)
- 08. [`Plan:` Interop](https://github.com/ipfs/testground/issues/138)

There is 1 other test plan that were created to test the functionality of Testground and to be used as a demo. The plan is: [smlbench](../plans/smlbench)

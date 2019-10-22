# TestGround testing Plan Specification

Each testing **Plan** contains:
- An **overview** of what we are looking to achieve with the test (roughly ~1 paragraph).
- **What we are looking to expect to be able to optimize** by running this test and therefore, a suggestion of what are The data points that must be gathered in order to assess if an improvement or regression has been made.-
- The **plan & test parameters**. This include both Network Parameters (e.g. Number of Nodes) and Image Parameters (e.g. bucket_size, bitswap strategy, etc)
- A Narrative for each Test that describes on how the network will set up itself (_Warm Up_ phase) and how the actors will play their multiple roles (in _Acts_).

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

The testing Plans linked below have been identified as the most valuable sets of tests to implement for the TestGround MVP. The goal of delivering a good characterization of the performance of IPFS in specific areas that can bolster our confidence level in shipping high quality releases of go-ipfs. Once the first 10 test Plans are written and TestGround fully deployed and operation, we will continue expanding the types of tests available.

- 01. [`Plan:` Chewing strategies for Large DataSets](../plans/chew-large-datasets)
- 02. [`Plan:` Data Transfer of Random DataSets (Bitswap/GraphSync)](../plans/data-transfer-datasets-random)
- 03. [`Plan:` Data Exchange with Datasets of Interest (BitSwap/GraphSync)](../plans/data-transfer-datasets-interest)
- 04. [`Plan:` Nodes Connectivity (Transports, Hole Punching, Relay)](../plans/nodes-connectivity)
- 05. [`Plan:` Providing Content (Content Routing / DHT)](../plans/providing-content)
- 06. [`Plan:` Message Delivery (PubSub)](../plans/message-delivery)
- 07. [`Plan:` Naming (IPNS & its multiple routers)](../plans/naming)
- 08. `TBD`
- 09. `TBD`
- 10. `TBD`

There are 2 other test plans that were created to test the functionality of TestGround and to be used as a demo. These are:

- [smlbench](../plans/smlbench)
- [dht](../plans/dht)
# `Plan:` Data Exchange with Datasets of Interest (BitSwap/GraphSync)

This test resembles the previous one (Data Transfer of Random DataSets (Bitswap/GraphSync)) with a twist. It focuses the attention to the experience will have when using IPFS to access popular and/or interesting datasets. The current datasets that this test plan contemplates are:

- Wikipedia Mirror
- NPM clone
- ImageNet

## What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  - To compute this, capture: file size, number of nodes in the IPLD graph, time to fetch it from first block to completion.
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The number of duplicated blocks received vs. total block count
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)

## Plan Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - Ran with with an arbitraty amount of nodes (from 10 to 1000000) - N
- **Image Parameters**
  - Number of nodes with full replica of the dataset initially (from 1 to 10) - M
  - Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

## Tests

### `Test:` _NAME_

- **Test Parameters**
  - n/a
- **Narrative**
  - **Warm up**
    - Boot N nodes
    - Connect each node to the node next to it (hash ring)
    - Run multiple DHT random-walk queries to populate the finger tables
    - Run a discovery service provided by Redis (to ensure that every node keeps getting at least one another node to connect)
    - Load the datasets in M nodes
  - **Act I** - Access Wikipedia
    - %W of the nodes access Wikipedia
  - **Act II** - Access NPM
    - %W of the nodes access NPM
  - **Act III** - Access ImageNet
    - %W of the nodes access ImageNet

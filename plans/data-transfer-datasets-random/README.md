# `Plan:` Data Transfer of Random DataSets (Bitswap/GraphSync)

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

Create an environment in which data transfer is stress tested. This test is not about content discovery or connectivity, it is assumed that all nodes are dialable by each other and that these are executed in an homogeneous network (same CPU, Memory, Bandwidth).

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
  - Single Image - The go-ipfs commit that is being tested
    - Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)
  - `File Sizes` - An array of File Sizes to be tested (default to: `[1MB, 1GB, 10GB, 100GB, 1TB]`)
  - `Directory Depth` - An Array containing objects that describe how deep/nested a directory goes and the size of files that can be found throughout (default to `[{depth: 10, size: 1MB}, {depth: 100, size: 1MB}]`

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
    - Each node creates a dataset with random data following the parameters `File Sizes` and `Directory Depth`
    - The nodes are divided in 4 cohorts, A, B, C & D, which each contains a set of %25 of the nodes available without creating an overlap (recommended to use a number of nodes that is a multiple of 4 to simplify the reasoning at the end (i.e. not having a situation in which a transfer of the file was instant))
  - **Act I**
    - Cohort B fetches the files created from Cohort A
  - **Act II**
    - Cohort C fetches the files created from Cohort A (expected to see speed improvements given that %50 of the network will have the file)
  - **Act III**
    - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
  - **Act III**
    - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
  - **Act IV**
    - Cohort A, B & C fetch the files created from Cohort D


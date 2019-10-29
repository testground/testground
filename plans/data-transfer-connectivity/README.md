# `Plan:` Data Transfer with variable connectivity (Bitswap/GraphSync)

Create an environment in which latency and connectivity vary while a file is being transferred.

## What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  To compute this, capture:
  - file size
  - time from the first leech request to the last leech block receipt
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The byte size of duplicated blocks received vs. total blocks received
- (Minimize) The total time to transfer all data to all leeches
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)

## Plan Parameters

- **Network Parameters**
  - `Node count` - The number of nodes
  - `Node lifetime average` - The average amount of time a seed node stays online
  - `Node lifetime variance` - The variance over the average lifetime
  - `Latency Average` - The average latency of connections in the system
  - `Latency Variance` - The variance over the average latency

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

## Tests

### `Test:` _NAME_

- **Test Parameters**
  - n/a
- **Narrative**
  - **Warm up**
    - Boot N nodes
    - Connect all nodes to each other
    - Create a large file of random data
    - Distribute the file to all seed nodes
  - **Benchmark**
    - One leech node requests the file

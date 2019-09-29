# List of Test Plans to be written for Test Ground MVP

The following test cases have been identified as the initial set of tests to implement using testground, with the goal of delivering a good characterization of the performance of IPFS in specific areas, while still being possible to deliver the tests within a 1 month implementation period.

Each test presents:
- What we are looking to optimize (aka the things to monitor and measure so that we can take conclusions out of the test)
- The execution variants (aka the knobs that should be at our avail to test IPFS in different ways
- The Test Narrative (the description of what should happen). Each Narrative has a _Warm Up_ phase that creates the envinronment in which we want to run our tests. Each Narrative contains 1 or move _Waves_, each Wave starts only after the previous has completed.

## Test Plans

### 1. Data Transfer of Random DataSets (Bitswap/GraphSync)

Create an envinroment in which data transfer is stress tested. This test is not about content discovery or connectivity, it is assumed that all nodes are dialable by each other and that these are executed in an homogeneus network (same CPU, Memory, Bandwidth).

#### What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  - To compute this, capture: file size, number of nodes in the IPLD graph, time to fetch it from first block to completion.
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The number of duplicated blocks received vs. total block count
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)

#### Execution Variants 

This test is complete if one can:

- Ran with with an arbitraty amount of nodes (from 10 to 1000000) - N
- Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

#### Test Narrative

- **Warm up** 
  - Boot N nodes
  - Connect each node to the node next to it (hash ring)
  - Run multiple DHT random-walk queries to populate the finger tables
  - Run a discovery service provided by Redis (to ensure that every node keeps getting at least one another node to connect)
  - Each node creates a dataset with random data of:
    - Single File
      - 1MB
      - 1GB
      - 10GB
      - 100GB
      - 1TB
    - Directory
      - 10 level nested directory with many 1MB
      - 10 level nested directory with many 10GB
      - 100 level nested directory with many 1MB
  - The nodes are divided in 4 cohorts, A, B, C & D, which each contains a set of %25 of the nodes available without creating an overlap (recommended to use a number of nodes that is a multiple of 4 to simplify the reasoning at the end (i.e. not having a situation in which a transfer of the file was instant))
- **Wave I**
  - Cohort B fetches the files created from Cohort A
- **Wave II**
  - Cohort C fetches the files created from Cohort A (expected to see speed improvements given that %50 of the network will have the file)
- **Wave III**
  - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
- **Wave III**
  - Cohort D fetches the files created from Cohort A (expected to see speed improvements given that %75 of the network will have the file)
- **Wave IV**
  - Cohort A, B & C fetch the files created from Cohort D


### 2. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 



### 3. ???


#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 


### 4. ???


#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 5. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 6. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 7. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 8. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 9. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

### 10. ???

#### What is being optimized (min/max, reach)

#### Execution Variants 

#### Test Narrative

- **Warm up** 
  - 
- **Wave I**
  - 
- **Wave II**
  - 
- **Wave III**
  - 

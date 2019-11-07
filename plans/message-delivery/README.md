# `Plan:` Message Delivery (PubSub)

## What is being optimized (min/max, reach)

- (Minimize) The time between publisher publishing a message and all subscribers receiving it
- (Minimize) Overall bandwidth consumed (either by topology forming or message routing)
- (Minimize) Duplicated messages received
- (Maximize) Success of delivery

## Plan Parameters

- **Network Parameters**
  - `N` - Number of nodes that are spawn for the test (from 10 to 1000000)
- **Image Parameters**
  - GO_IPFS_VERSION - The go-ipfs version number or commit that is being tested
  - Image Resources CPU & Ram

## Tests

### `Test:` A Stable Network

- **Test Parameters**
  - PUBSUB_ALGORITHM - The pubsub algorithm to be tested (floodsub or gossip sub)
  - P_PUBLISHERS - Number of nodes that are publishing messages
  - T_TOPICS - The number of topics created. A publisher can decide to publish in 1 or more topics
  - S_SUBSCRIBERS - Number of nodes that are subscribing 
- **Narrative**
  - **Warm up**
    - Spin up `N` nodes
    - Connect each node to a previous node (ring shape network)
  - **Act I**
    - b
  - **Act II**
    - c

### `Test:` Network with High Churn

- **Test Parameters**
  - PUBSUB_ALGORITHM - The pubsub algorithm to be tested (floodsub or gossip sub)
  - T_TOPICS - The number of topics created. A publisher can decide to publish in 1 or more topics
  - P_PUBLISHERS - Number of nodes that are publishing messages
  - P_PUBLISHERS_CHURN - The % of publishers that will go down and up
  - P_PUBLISHERS_CHURN_DELAY - The interval between each node online -> offline -> online cycle
  - S_SUBSCRIBERS - Number of nodes that are subscribing 
  - P_SUBSCRIBERS_CHURN - The % of subscribers that will go down and up
  - P_SUBSCRIBERS_CHURN_DELAY - The interval between each node online -> offline -> online cycle
- **Narrative**
  - **Warm up**
    - a
  - **Act I**
    - b
  - **Act II**
    - c
  - **Act III**
    - d


### `Test:` Chaotic Network (netsplits and rejoins)

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

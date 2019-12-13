# `Plan:` The go-libp2p DHT behaves

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

IPFS can safely rely on the latest DHT upgrades by running go-libp2p DHT tests directly

## What is being optimized (min/max, reach)

- (Minimize) Numbers of peers dialed to as part of the call
- (Minimize) Number of failed dials to peers
- (Minimize) Time that it took for the test
- (Minimize) Routing efficiency (# of hops (XOR metric should decrease at every step))

## Plan Parameters

- **Builder Parameters**
  - Single Image - The go-libp2p commit that is being tested
- **Runner Paramegers**
  - Image Resources CPU & Ram
- **Network Parameters**
  - `instances` - Number of nodes that are spawned for the test (from 10 to 1000000)

## Tests

### `Test:` Find peers

> Test the Peer Routing efficiency

- **Test Parameters**
  - `auto-refresh` - Enable autoRefresh (equivalent to running random-walk multiple times automatically) (true/false, default: false)
  - `random-walk` - Run random-walk manually 5 times (true/false, default: false)
  - `bucket-size` - Kademlia DHT bucket size (default: 20)
  - `n-find-peers` - Number of times a Find Peers call is executed from each node (picking another node PeerId at random that is not yet in our Routing table) (default: 1)
- **Narrative**
  - **Warm up**
    - All nodes boot up
    - Each node as it boots up, connects to the node that previously joined
    - Nodes ran 5 random-walk queries to populate their Routing Tables
  - **Act I**
    - Each node calls Find Peers `n-find-peers` times

### `Test:` Find providers

> Content Routing, finding providers

- **Test Parameters**
  - `auto-refresh` - Enable autoRefresh (equivalent to running random-walk multiple times automatically) (true/false, default: false)
  - `random-walk` - Run random-walk manually 5 times (true/false, default: false)
  - `bucket-size` - Kademlia DHT bucket size (default: 20)
  - `p-providing` - Percentage of nodes providing a record
  - `p-resolving` - Percentage of nodes trying to resolve the network a record
  - `p-failing` - Percentage of nodes trying to resolve a record that hasn't been provided
- **Narrative**
  - **Warm up**
    - All nodes boot up
    - Each node as it boots up, connects to the node that previously joined
    - Nodes ran 5 random-walk queries to populate their Routing Tables
  - **Act I**
    - `p-providing` of the nodes provide a record and store its key on redis
  - **Act II**
    - `p-resolving` of the nodes attempt to resolve the records provided before
    - `p-failing` of the nodes attempt to resolve records that do not exist


### `Test:` Provide stress

> Content Routing, providing a ton of records

- **Test Parameters**
  - `auto-refresh` - Enable autoRefresh (equivalent to running random-walk multiple times automatically) (true/false, default: false)
  - `random-walk` - Run random-walk manually 5 times (true/false, default: false)
  - `bucket-size` - Kademlia DHT bucket size (default: 20)
  - `n-provides` - The number of provide calls that are done by each node
  - `i-provides` - The interval between each provide call (in seconds)
- **Narrative**
  - **Warm up**
    - All nodes boot up
    - Each node as it boots up, connects to the node that previously joined
    - Nodes ran 5 random-walk queries to populate their Routing Tables
  - **Act I**
    - Each node calls Provide for `i-provides` until it reaches a total of `n-provides`

### `Test:` Provider record saturation

> The libp2p DHT activates Load Balancing techniques once it understand that a node is owning a popular segment of the address space and with that, storing a larger amount of records that what it can handle. This test is designed to verify that libp2p is actively mitigating that by distributing that load with other nodes.

We need to test:
- Records are replicated to more nodes in the edges of the N closest nodes, once those nodes store a large amount of records (beyond what their computing resources enable)
- Same as above, but in the inverse direction, that is: records are not replicated to more nodes, once the load is reduced
- Records are replicated to more nodes in the edges of the N closest nodes, once those records are heavily requested (beyond what their bandwidth enables the N closest nodes to serve them)
- Same as above, but in the inverse direction, that is: records are not replicated to more nodes, once the load is reduced

### `Test:` Switch modes (client, full)

`blocked` - This test is blocked until support for simulated NAT is done.

We need to test:
- The peers switch to client or full node depending on their network conditions

### `Test:` Persistent routing table

We need to test:
- A peer recovers their routing table even after a shutdown and reboot

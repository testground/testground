# `Plan:` Naming (IPNS & its multiple routers)

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

## What is being optimized (min/max, reach)

- (Minimize) The time it takes to publish an IPNS record.
- (Minimize) The time it takes to resolve an IPNS record the first time.
- (Minimize) The time it takes to resolve an IPNS record subsequent times.

### Plan Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - `N` - Number of nodes that are spawn for the test (from 10 to 1000000)
- **Image Parameters**
  - Single Image - The go-ipfs commit that is being tested
  - Image Resources CPU & Ram

## Tests

### `Test:` Publish/Resolve

- **Test Parameters**
  - `EnableIPNSPubSub` - Whether or not IPNS over pubsub is enabled.
  - `Resolvers` - Number nodes resolving the key.
  - `Delay` - Delay between resolvers resolving the keys
  - `Connect` - how to connect the nodes. These can be set independently.
    - `Bootstrap` - Number of "bootstrap" nodes. All nodes will connect to these bootstrap nodes on start.
    - `Random` - Number of random nodes to connect each node to.
    - `Spanning` - Number of "resolvers" each resolver should be connected to.
- **Narrative**
  - **Warm up**
    - Start and connect the nodes as specified.
  - **Act I**
    - Publish the IPNS record and wait for publish to return.
  - **Act II**
    - Staggered by `Delay`, each resolver should try to resolve the IPNS key.
  - **Act III**
    - Concurrently:
      - The publisher publishes a new value.
      - The resolvers repeatedly resolve the IPNS key until they see the new record.

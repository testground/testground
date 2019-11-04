# `Plan:` The go-libp2p DHT behaves

IPFS can safely rely on the latest DHT upgrades by running go-libp2p DHT tests directly

## What is being optimized (min/max, reach)

- (Minimize) Numbers of peers dialed to as part of the call
- (Minimize) Number of failed dials to peers
- (Minimize) Time that it took for the test
- (Minimize) Routing efficiency (# of hops (XOR metric should decrease at every step))

## Plan Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - `N` - Number of nodes that are spawn for the test (from 10 to 1000000)
- **Image Parameters**
  - Single Image - The go-libp2p commit that is being tested
  - Image Resources CPU & Ram

## Tests

### `Test:` Find Peers

- **Test Parameters**
  - `random-walk` - Automatic random-walk On/Off
- **Narrative**
  - **Warm up**
    - a
  - **Act I**
    - b

### `Test:` Find Providers

- **Test Parameters**
  - `random-walk` - Automatic random-walk On/Off
- **Narrative**
  - **Warm up**
    - a
  - **Act I**
    - b

### `Test:` Providing Records

- **Test Parameters**
  - `random-walk` - Automatic random-walk On/Off
- **Narrative**
  - **Warm up**
    - a
  - **Act I**
    - b

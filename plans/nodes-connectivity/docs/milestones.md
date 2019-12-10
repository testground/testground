# Milestones

From: [#96](https://github.com/ipfs/testground/issues/96#issuecomment-562877707)

## Milestone 1

Implement the Circuit Relay Testing, leveraging sidecar to create the following scenarios:
- Same LAN
- Different Subnets that are not routable to each other

You can draw inspiration from https://github.com/ipfs/interop/blob/master/test/circuit/all.js for the Test Cases

## Milestone 2

Ideally, what we want to have is a grid in which the columns are the Network scenario and the rows are the possible configurations.

Example (for interpretation):
```
| PeerA        | PeerB                 | Scenario 1    | Scenario 2 | 
| ----------- | --------------   | ------------ | -- |
| TCP+uTP   | TCP+QUIC         | ✅                 | ❌ |
| TCP+Relay | WebRTC+Relay | ✅                 | ✅ |
```

Milestone 2 should be about coming up with the grid referenced above and start hitting each test case for each cell in the grid.
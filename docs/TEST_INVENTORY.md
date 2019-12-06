# Available Test Suites accross {go, js}-IPFS and libp2p land

Complete, in-progress, and TBD.

## Sharness

* Status: ~50% Complete
* Language: Bash
* Type: Integration/API
* Target: Js & Go have separate suites. Work has been done to unify them but that was never completed.
* Where: CI

## Go CoreAPI Tests

* Status: ~25% Complete
* Language: Go
* Type: Integration/API
* Target: Go (for now), Js (future)
* Where: CI

## JS CoreAPI Tests

* Status: ~86% Complete
* Language: JS
* Type: Integration/API
* Target: JS & Go
* Where: CI

## ipfs/interop

* Status: Interoperability between IPFS implementations (Go & JS)
* Language: JS
* Type: Integration
* Target: JS & Go
* Where:Testing Infra (future)

## ipfs/benchmarks
> Small-scale benchmarks (max 5 local nodes)

* Status: Benchmarks between js-ipfs & go-ipfs (captures DTrace probes, creates flamegraphs and bubleprofs)
* Language: JS
* Type: Integration
* Target: JS & Go
* Where: Benchmarking Infra

A test framework and a series of test cases written in JS. It captures traces analysable by Node Clinic, and records metrics and test runs in InfluxDB. Capable of orchestrating go daemons and embedded JS nodes. JS nodes are embedded in the V8 runtime where the test is taking place, whereas Go nodes are spawned as standalone processes.

Existing test cases are simple and are mostly concerned with measuring time between commands as observed by the client of the APIs, e.g. instantiate a JS node, add a file, record the time elapsed.

Note: Clinic Doctor traces appear to be collected of the test harness itself, not of each JS node separately. The lack of profile isolation may produce inaccurate/unreliable results, because:

1. it contains test runtime overhead – and –
2. if a test case instantiates multiple JS nodes, it conflates all traces in a single profiling session.

<table>
  <tr>
   <td>
<strong>Test suite</strong>
   </td>
   <td><strong>Single node</strong>
   </td>
   <td><strong>Two nodes</strong>
   </td>
   <td><strong>Multiple nodes</strong>
   </td>
  </tr>
  <tr>
   <td><strong>Node initialization</strong>
   </td>
   <td>Node initialization
   </td>
   <td>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><strong>Adding files</strong> \
variants: balanced DAG, trickle DAG
   </td>
   <td>Add small file
<p>
Add many small files
<p>
Add large file
   </td>
   <td>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><strong>Catting files</strong> \
variants: transport [tcp/websocket/webrtc] | multiplexer [mplex] | security [secio/none]
   </td>
   <td>Cat small file
<p>
Cat large file
   </td>
   <td>Cat small file
<p>
Cat large file
   </td>
   <td>Cat small file
<p>
Cat large file
   </td>
  </tr>
  <tr>
   <td><strong>MFS tests</strong>
   </td>
   <td>MFS write small file
<p>
MFS write many small files (10k)
<p>
MFS write large file
<p>
MFS write to dir with < 1,000 files
<p>
MFS write to dir with > 1,000 files
<p>
MFS write to deeply nested dir
<p>
MFS read a small file
<p>
MFS read a large file
<p>
MFS cp a file
<p>
MFS mv a file
<p>
MFS rm a file
<p>
MFS stat a file
   </td>
   <td>
   </td>
   <td>
   </td>
  </tr>
</table>

## Go Integration Benchmarks

* Status: They work but we don't run them automatically.
* Language: Go
* Type: Benchmark
* Target: Go Only
* Where: Local (now), Testing Infra (future)

## Libp2p Interop Tests

* Status: Active (Vasco). Run manually?
* Language: JS
* Type: Integration/API
* Target: JS & Go
* Where: Local? (has .travis.yml, but doesn’t seem to be active)

## Kubernetes-IPFS

* Status: Mothballed? Last active 2018
* Language: YAML-based DSL ([example](https://github.com/ipfs/kubernetes-ipfs/blob/master/tests/moving-medium.yml)), implemented with Go
* Type: Integration / Benchmark?
* Target: Go
* Where: Local (Minikube), Kubernetes

## Filecoin Automation & System Toolkit (FAST) + localnet

* Status: Active
* Language: Go
* Type: Integration / Benchmark?
* Target: Go
* Where: Local, CI?

## PeerPad / peer-base Tests

* Status: Active
* Language: JS
* Type: Integration / Load (OpenCensus/Jaeger support on branch)
* Target: JS
* Where: Local
* Want: WebRTC / NAT testing, Puppeteer / WebDriver + UI screen recordings, large CRDT scalability, replace \*-star with rendezvous, offline testing

## Package Manager Benchmarks

* Status: Not started yet
* Language: Go
* Type: Integration / Benchmark
* Target: Go
* Where: Local, Testing Infra
* Want: quick content discovery, large file ingests and transfers, IPNS - pubsub, pinning, multiwriter

## IPFS Cluster Testing

* Status: Not started (it was Kubernetes-ipfs)
* Language: Go
* Type: -
* Want: IPTB/Kubernetes-IPFS like tests (end to end, on a cluster swarm, using the Cluster API to perform operations) that gather key metrics and traces. It should be quite aligned with how a swarm of ipfs daemons is tested.

## Canary tests against production

**DHT: Find peer in public network**

For 16 peers connected to the public network and bootstrapped against a single bootstrapper out of a list of bootstrappers B[0..n], where B[i] is picked in round robin fashion. For each peer P, and all other peers p[i]:

1. ipfs dht findpeer p[i].
2. Record the time it took.

See [https://github.com/jbenet/dht-simple-tests](https://github.com/jbenet/dht-simple-tests).

**DHT: Add a file -- time to first provider record, peers queried**

For a single node P, generate a random 1mb file F, and run ipfs add F. Watch the IPFS event logs and count the number of peers queried, and the time it took until the first provider record was published.

**End-to-end add & get**

1. Generate a random file F of size 10mb.
2. From node A, ipfs add the file. Get the CID.
3. From node B, ipfs get that CID.
4. Time the whole process.

## Reproducible network tests

We’re aiming to support 100.000+ nodes.

Tests need to be fully reproducible. This means:

*   Network needs to be tightly controlled and possibly mocked, simulated.
  *   See points below.
    *   Certain scenarios will require persistent peer IDs (e.g. content or peer routing)
    *   Certain scenarios will require fixed choreographies, e.g. if we want the routing table to be the same across all runs of test scenario “foo”, we’ll need to connect nodes with fixed identities, in fixed sequences. Otherwise the results may be unstable.
    *   Consider a test fixtures repository: a service that store pools of test fixtures (e.g. peer identities), that nodes can query “check out” a test fixture satisfying facets A, B, C, and check them back in when they’re done.

Defining network topologies, mainly to simulate NAT’ted, relayed nodes, signalling use cases, etc. Otherwise it’s safe to assume a _fully connectable_ paradigm.

Simulating network conditions via “network blueprints” -- nodes will be running in a distributed environment (we need to horizontally scale to support 100.000+ nodes).
*   There’s actually a real network involved, on top of which we need to overlay network conditions.
*   Latency, packet loss, jitter, bandwidth limitations on uplink and downlink, etc.

## PoC Large-scale private network test (Cole)

DHT-based. Raúl will post a link to Juan’s DHT canary test for inspiration.

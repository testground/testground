# Testground Overview

Testground is a tool for testing next generation P2P applications (i.e. Filecoin, IPFS, libp2p & others).

## Testground Architecture

Testground uses a client-server architecture. The Testground client talks to the Testground daemon, which builds a given test plan, and runs it. The Testground client and daemon can run on the same system, or you can connect a Testground client to a remote Testground daemon. The Testground client and daemon communicate using a REST API, over UNIX sockets or a network interface.

### The Testground daemon

The Testground daemon listens for Testground API requests and manages Testground test plan builds and runs.

### The Testground client

The Testground client is the primary way that users interact with Testground. When you use commands such as `testground run`, the client sends these commands to daemon, which executes them. The `testground` command uses the Testground API.

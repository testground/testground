# Networking

All testground runners _except_ for the local runner have two networks: a
control and a data network.

* Test instances communicate with each other over the data network.
* Test instances communicate with the sync service, and _only_ the sync service,
  over the control network.

The local runner will use your machine's local network interfaces. For now, this runner doesn't support traffic shaping.

## Control Network

The "control network" runs on 192.18.0.1/16 and should only be used to
communicate with the sync service.

After the sidecar is finished [initializing the
network](https://github.com/ipfs/testground/blob/master/docs/SIDECAR.md#initialization),
it should be impossible to use this network to communicate with other nodes.
However, a good test plan should avoid listening on and/or announcing this
network _anyways_ to ensure that it doesn't interfere with the test.

## Data Network

The "data network", used for all inter-instance communication, will be assigned
a B block in the IP range 16.0.0.0 - 32.0.0.0. Given the B block X.Y.0.0,
X.Y.0.1 is always the gateway and shouldn't be used by the test.

The subnet used will be passed to the test instance via the runtime environment
(as `TestSubnet`).

You can change your IP address (within this range) at any time [using the
sidecar](https://github.com/ipfs/testground/blob/master/docs/SIDECAR.md#ip-addresses).

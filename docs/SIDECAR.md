# Sidecar

Before you read this, read the
[networking](https://github.com/ipfs/testground/blob/master/docs/NETWORKING.md)
documentation.

In testground, a test instance can configure its own network (IP address,
jitter, latency, bandwidth, etc.) by writing a `NetworkConfig` to the sync
service, then waiting for the specified `State` to be set.

## Worked Example

### Imports

For this example, you'll need the following packages:

```go
import (
	"net"
	"os"
	"reflect"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)
```

### Pre-Check

First, check to make sure the sidecar is even available. At the moment, it's
only available on docker-based runners. If it's not available, just skip any
networking config code and proceed.

```go
// runenv is the test instances run environment (runtime.RunEnv).
if !runenv.TestSidecar {
    return
}
```

### Initialization

First, wait for the sidecar to initialize the network.

```go
if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
    runenv.Abort(err)
    return
}
```

If you don't want to customize the network (set IP addresses, latency, etc.),
you can stop here.

### Hostname

The sidecar identifies test instances by their hostname.

```go
hostname, err := os.Hostname()
if err != nil {
    runenv.Abort(err)
    return
}
```

### Configure: Create

Once the network is ready, you'll need to actually _configure_ your network.

```go
config := sync.NetworkConfig{
    // Control the "default" network. At the moment, this is the only network.
    Network: "default",

    // Enable this network. Setting this to false will disconnect this test
    // instance from this network. You probably don't want to do that.
    Enable:  true,
}
```

#### Traffic Shaping

To "shape" traffic, set the `Default` `LinkShape`. You can use this to set
latency, bandwidth, jitter, etc.

```go
config.Default = sync.LinkShape{
    Latency:   100 * time.Millisecond,
    Bandwidth: 1 << 20, // 1Mib
}
```

NOTE: This sets _egress_ (outbound) properties on the link. These settings must
be symmetric (applied on both sides of the connection) to work properly (unless
asymmetric bandwidth/latency/etc. is desired).

NOTE: Per-subnet traffic shaping is a desired but unimplemented feature.
The sidecar will reject configs with per-subnet rules set in
`NetworkConfig.Rules`.

#### IP Addresses

If you don't specify an IPv4 address when configuring your network, your test
instance will keep the default assignment. However, if desired, a test instance
can change its IP address at any time.

First, you'll need some kind of unique sequence number to ensure you don't pick
conflicting addresses. If you don't already have some form of unique sequence
number at this point in your tests, use the sync service to get one:

```go
seq, err := writer.Write(&sync.Subtree{
    GroupKey:    "ip-allocation",
    PayloadType: reflect.TypeOf(""),
    KeyFunc: func(val interface{}) string {
        return val.(string)
    },
}, hostname)
if err != nil {
    runenv.Abort(err)
    return
}
```

Once you have a sequence number, you can set your IP address from one of the
available subnets:

```go
// copy the test subnet.
config.IPv4 = &*runenv.TestSubnet
// Use the sequence number to fill in the last two octets.
//
// NOTE: Be careful not to modify the IP from `runenv.TestSubnet`.
// That could trigger undefined behavior.
ipC := byte((seq >> 8) + 1)
ipD := byte(seq)
config.IPv4.IP = append(config.IPv4.IP[0:2:2], ipC, ipD)
```

NOTE: You cannot currently set an IPv6 address.

### Configure: Apply

Network configurations are applied as follows:

1. The test instance sets a "state" that should be signaled when the new
   configuration has been applied.

```go
config.State = "network-configured"
```

2. The test instance writes the new network configuration to the sync service.

```go
_, err = writer.Write(sync.NetworkSubtree(hostname), &config)
if err != nil {
    runenv.Abort(err)
    return
}
```

3. The test instance _waits_ on the specified "state" (barrier). Note: the
   following example will wait for _all_ test instances to finish configuring
   their networks.

```go
err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
if err != nil {
    runenv.Abort(err)
    return
}
```

The sidecar follows the complementary steps:

1. The sidecar reads the network configuration from the sync service.
2. The sidecar applies the network configuration.
3. The sidecar signals the configured "state".

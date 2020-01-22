//+build linux

package sidecar

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ipfs/testground/pkg/dockermanager"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

const (
	controlNetworkIfname = "eth0"
	dataNetworkIfname    = "eth1"
	podCidr              = "100.96.0.0/11"
)

type K8sInstanceManager struct {
	redis   net.IP
	manager *dockermanager.Manager
}

func NewK8sManager() (InstanceManager, error) {
	redisHost := os.Getenv(EnvRedisHost)

	redisIp, err := net.ResolveIPAddr("ip4", redisHost)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve redis: %w", err)
	}

	docker, err := dockermanager.NewManager()
	if err != nil {
		return nil, err
	}

	return &K8sInstanceManager{
		manager: docker,
		redis:   redisIp.IP,
	}, nil
}

func (d *K8sInstanceManager) Manage(
	ctx context.Context,
	worker func(ctx context.Context, inst *Instance) error,
) error {
	return d.manager.Manage(ctx, func(ctx context.Context, container *dockermanager.Container) error {
		logging.S().Debugw("got container", "container", container.ID)
		inst, err := d.manageContainer(ctx, container)
		if err != nil {
			return fmt.Errorf("failed to initialise the container: %w", err)
		}
		if inst == nil {
			logging.S().Debugw("ignoring container", "container", container.ID)
			return nil
		}
		logging.S().Debugw("managing container", "container", container.ID)
		err = worker(ctx, inst)
		if err != nil {
			return fmt.Errorf("container worker failed: %w", err)
		}
		return nil
	})
}

func (d *K8sInstanceManager) Close() error {
	return d.manager.Close()
}

func (d *K8sInstanceManager) manageContainer(ctx context.Context, container *dockermanager.Container) (inst *Instance, err error) {
	// Get the state/config of the cluster
	info, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	if !info.State.Running {
		return nil, fmt.Errorf("not running")
	}

	// Construct the runtime environment
	runenv, err := runtime.ParseRunEnv(info.Config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run environment: %w", err)
	}

	if !runenv.TestSidecar {
		return nil, nil
	}

	// Initialise CNI config
	cninet := libcni.NewCNIConfig(filepath.SplitList("/host/opt/cni/bin"), nil)

	// Get a netlink handle.
	nshandle, err := netns.GetFromPid(info.State.Pid)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup the net namespace: %s", err)
	}
	defer nshandle.Close()

	netlinkHandle, err := netlink.NewHandleAt(nshandle)
	if err != nil {
		return nil, fmt.Errorf("failed to get handle to network namespace: %w", err)
	}

	defer func() {
		if err != nil {
			netlinkHandle.Delete()
		}
	}()

	// Finally, construct the network manager.
	network := &K8sNetwork{
		netnsPath:   fmt.Sprintf("/proc/%d/ns/net", info.State.Pid),
		cninet:      cninet,
		container:   container,
		subnet:      runenv.TestSubnet.String(),
		nl:          netlinkHandle,
		activeLinks: make(map[string]*k8sLink),
	}

	// Remove all routes but redis and the data subnet

	// We've found a control network (or some other network).
	controlLink, err := netlinkHandle.LinkByName(controlNetworkIfname)
	if err != nil {
		return nil, fmt.Errorf("failed to get link by name %s: %w", controlNetworkIfname, err)
	}

	// Get the routes to redis. We need to keep these.
	redisRoutes, err := netlinkHandle.RouteGet(d.redis)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route to redis: %w", err)
	}

	if len(redisRoutes) != 1 {
		return nil, fmt.Errorf("expected to get only one route to redis, but got %v", len(redisRoutes))
	}

	redisRoute := redisRoutes[0]

	controlLinkRoutes, err := netlinkHandle.RouteList(controlLink, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes for control link %s", controlLink.Attrs().Name)
	}

	gw := redisRoute.Gw

	// Remove the original routes
	for _, route := range controlLinkRoutes {
		if route.Dst != nil && route.Dst.String() == podCidr {
			logging.S().Debugw("removing route", "route.Src", route.Src.String(), "route.Dst", podCidr, "gw", route.Gw.String(), "container", container.ID)

			if err := netlinkHandle.RouteDel(&route); err != nil {
				return nil, fmt.Errorf("failed to delete pod cidr route: %v", err)
			}

			continue
		}

		if route.Dst != nil && route.Dst.Contains(gw) {
			if err := netlinkHandle.RouteDel(&route); err != nil {
				return nil, fmt.Errorf("failed to delete route while restricting gw route: %v", err)
			}

			route.Dst.IP = gw
			route.Dst.Mask = net.CIDRMask(32, 32)

			if err := netlinkHandle.RouteAdd(&route); err != nil {
				return nil, fmt.Errorf("failed to add route while restricting gw route: %v", err)
			}
		}

		if route.Dst == nil && route.Src == nil {
			// if default route, get the gw and add a route for DNS
			dnsRoute := route
			dnsRoute.Src = nil
			dnsRoute.Dst = &net.IPNet{
				IP:   net.IPv4(100, 64, 0, 10),
				Mask: net.CIDRMask(32, 32),
			}

			logging.S().Debugw("adding dns route", "container", container.ID)
			if err := netlinkHandle.RouteAdd(&dnsRoute); err != nil {
				return nil, fmt.Errorf("failed to add dns route to pod: %v", err)
			}

			logging.S().Debugw("removing default route", "container", container.ID)
			if err := netlinkHandle.RouteDel(&route); err != nil {
				return nil, fmt.Errorf("failed to drop default route: %v", err)
			}
		}
	}

	// Add specific routes to redis if redis uses this link.
	if err := netlinkHandle.RouteAdd(&redisRoute); err != nil {
		return nil, fmt.Errorf("failed to add redis route: %v", err)
	}

	return NewInstance(runenv, info.Config.Hostname, network)
}

type k8sLink struct {
	*NetlinkLink
	IPv4, IPv6 *net.IPNet

	rt      *libcni.RuntimeConf
	netconf *libcni.NetworkConfigList
}

type K8sNetwork struct {
	container   *dockermanager.Container
	activeLinks map[string]*k8sLink
	nl          *netlink.Handle
	cninet      *libcni.CNIConfig
	subnet      string
	netnsPath   string
}

func (n *K8sNetwork) Close() error {
	return nil
}

func (n *K8sNetwork) ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error {
	if cfg.Network != "default" {
		return errors.New("configured network is not default")
	}

	link, online := n.activeLinks[cfg.Network]

	// Are we _disabling_ the network?
	if !cfg.Enable {
		// Yes, is it already disabled?
		if online {
			// No. Disconnect.
			if err := n.cninet.DelNetworkList(ctx, link.netconf, link.rt); err != nil {
				return fmt.Errorf("when 6: %w", err)
			}
			delete(n.activeLinks, cfg.Network)
		}
		return nil
	}

	if online && ((cfg.IPv6 != nil && !link.IPv6.IP.Equal(cfg.IPv6.IP)) ||
		(cfg.IPv4 != nil && !link.IPv4.IP.Equal(cfg.IPv4.IP))) {
		// Disconnect and reconnect to change the IP addresses.
		logging.S().Debugw("disconnect and reconnect to change the IP addr", "cfg.IPv4", cfg.IPv4, "link.IPv4", link.IPv4.String(), "container", n.container.ID)
		//
		// NOTE: We probably don't need to do this on local docker.
		// However, we probably do with swarm.
		online = false
		if err := n.cninet.DelNetworkList(ctx, link.netconf, link.rt); err != nil {
			return fmt.Errorf("when 5: %w", err)
		}
		delete(n.activeLinks, cfg.Network)
	}

	// Are we _connected_ to the network.
	if !online {
		// No, we're not.
		// Connect.
		if cfg.IPv6 != nil {
			return errors.New("ipv6 not supported")
		}

		var (
			netconf *libcni.NetworkConfigList
			err     error
		)
		if cfg.IPv4 == nil {
			netconf, err = newNetworkConfigList("net", n.subnet)
		} else {
			netconf, err = newNetworkConfigList("ip", cfg.IPv4.String())
		}
		if err != nil {
			return fmt.Errorf("failed to generate new network config list: %w", err)
		}

		cniArgs := [][2]string{}                   // empty
		capabilityArgs := map[string]interface{}{} // empty

		rt := &libcni.RuntimeConf{
			ContainerID:    n.container.ID,
			NetNS:          n.netnsPath,
			IfName:         dataNetworkIfname,
			Args:           cniArgs,
			CapabilityArgs: capabilityArgs,
		}

		_, err = n.cninet.AddNetworkList(ctx, netconf, rt)
		if err != nil {
			return fmt.Errorf("failed to add network through cni plugin: %w", err)
		}

		netlinkByName, err := n.nl.LinkByName(dataNetworkIfname)
		if err != nil {
			return fmt.Errorf("failed to get link by name: %w", err)
		}

		// Register an active link.
		handle, err := NewNetlinkLink(n.nl, netlinkByName)
		if err != nil {
			return fmt.Errorf("failed to register new netlink: %w", err)
		}
		v4addrs, err := handle.ListV4()
		if err != nil {
			return fmt.Errorf("failed to list v4 addrs: %w", err)
		}
		if len(v4addrs) != 1 {
			return fmt.Errorf("expected 1 v4addrs, but received %d", len(v4addrs))
		}

		link = &k8sLink{
			NetlinkLink: handle,
			IPv4:        v4addrs[0],
			IPv6:        nil,
			rt:          rt,
			netconf:     netconf,
		}

		logging.S().Debugw("successfully adding an active link", "ipv4", link.IPv4, "container", n.container.ID)

		n.activeLinks[cfg.Network] = link
	}

	// We don't yet support applying per-subnet rules.
	if len(cfg.Rules) != 0 {
		return fmt.Errorf("TODO: per-subnet bandwidth rules not supported")
	}

	if err := link.Shape(cfg.Default); err != nil {
		return fmt.Errorf("failed to shape link: %w", err)
	}
	return nil
}

func (n *K8sNetwork) ListActive() []string {
	networks := make([]string, 0, len(n.activeLinks))
	for name := range n.activeLinks {
		networks = append(networks, name)
	}
	return networks
}

func newNetworkConfigList(t string, addr string) (*libcni.NetworkConfigList, error) {
	switch t {
	case "net":
		bytes := []byte(`
{
		"cniVersion": "0.3.0",
		"name": "weave",
		"plugins": [
				{
						"name": "weave",
						"type": "weave-net",
						"ipam": {
								"subnet": "` + addr + `"
						},
						"hairpinMode": true
				}
		]
}
`)
		return libcni.ConfListFromBytes(bytes)

	case "ip":
		bytes := []byte(`
{
		"cniVersion": "0.3.0",
		"name": "weave",
		"plugins": [
				{
						"name": "weave",
						"type": "weave-net",
						"ipam": {
								"ips": [
								  {
									  "version": "4",
										"address": "` + addr + `"
								  }
								]
						},
						"hairpinMode": true
				}
		]
}
`)
		return libcni.ConfListFromBytes(bytes)

	default:
		return nil, errors.New("unknown type")
	}
}

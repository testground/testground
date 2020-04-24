//+build linux

package sidecar

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/logging"

	"github.com/containernetworking/cni/libcni"
	"github.com/vishvananda/netlink"
)

type k8sLink struct {
	*NetlinkLink
	IPv4, IPv6 *net.IPNet

	rt      *libcni.RuntimeConf
	netconf *libcni.NetworkConfigList
}

type K8sNetwork struct {
	container   *docker.ContainerRef
	activeLinks map[string]*k8sLink
	nl          *netlink.Handle
	cninet      *libcni.CNIConfig
	subnet      string
	netnsPath   string
}

func (n *K8sNetwork) Close() error {
	n.nl.Delete()
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
			logging.S().Debugw("trying to add a link", "net", n.subnet, "container", n.container.ID)
			netconf, err = newNetworkConfigList("net", n.subnet)
		} else {
			logging.S().Debugw("trying to add a link", "ip", cfg.IPv4.String(), "container", n.container.ID)
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

		err = retry(3, 2*time.Second, func() error {
			_, err = n.cninet.AddNetworkList(ctx, netconf, rt)
			return err
		})
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

func getRedisRoute(handle *netlink.Handle, redisIP net.IP) (*netlink.Route, error) {
	redisRoutes, err := handle.RouteGet(redisIP)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve route to redis: %w", err)
	}

	if len(redisRoutes) != 1 {
		return nil, fmt.Errorf("expected to get only one route to redis, but got %v", len(redisRoutes))
	}

	redisRoute := redisRoutes[0]

	return &redisRoute, nil
}

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		logging.S().Warnw("got err, waiting to retry", "err", err.Error())

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

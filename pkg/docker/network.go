package docker

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func NewBridgeNetwork(ctx context.Context, cli *client.Client, name string, internal bool, labels map[string]string, config ...network.IPAMConfig) (id string, err error) {
	res, err := cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver:     "bridge",
		Attachable: true,
		Internal:   internal,
		Labels:     labels,
		IPAM: &network.IPAM{
			Config: config,
		},
	})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

func CheckBridgeNetwork(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) ([]types.NetworkResource, error) {
	opts := types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", name),
			filters.Arg("driver", "bridge"),
		),
	}
	return cli.NetworkList(ctx, opts)
}

func EnsureBridgeNetwork(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string, internal bool, config ...network.IPAMConfig) (id string, err error) {
	networks, err := CheckBridgeNetwork(ctx, ow, cli, name)
	if err != nil {
		return "", err
	}

	if len(networks) > 0 {
		network := networks[0]
		ow.Debugw("network found", "name", name, "id", network.ID)
		return network.ID, nil
	}

	return NewBridgeNetwork(ctx, cli, name, internal, nil, config...)
}

package docker

import (
	"context"

	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"

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

func EnsureBridgeNetwork(ctx context.Context, log *zap.SugaredLogger, cli *client.Client, name string, internal bool, config ...network.IPAMConfig) (id string, err error) {
	opts := types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", name),
			filters.Arg("driver", "bridge"),
		),
	}
	networks, err := cli.NetworkList(ctx, opts)
	if err != nil {
		return "", err
	}

	if len(networks) > 0 {
		network := networks[0]
		log.Debugw("network found", "name", name, "id", network.ID)
		return network.ID, nil
	}

	return NewBridgeNetwork(ctx, cli, name, internal, nil, config...)
}

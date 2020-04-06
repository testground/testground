package runner

import (
	"context"
	"path/filepath"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/healthcheck"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func localCommonHealthcheck(ctx context.Context, hh *healthcheck.Helper, cli *client.Client, ow *rpc.OutputWriter, controlNetworkID string, srcdir string, workdir string) {
	hh.Enlist("local-outputs-dir",
		healthcheck.CheckDirectoryExists(workdir),
		healthcheck.CreateDirectory(workdir),
	)

	// testground-control network
	hh.Enlist("control-network",
		healthcheck.CheckNetwork(ctx, ow, cli, controlNetworkID),
		healthcheck.CreateNetwork(ctx, ow, cli, controlNetworkID, network.IPAMConfig{Subnet: controlSubnet, Gateway: controlGateway}),
	)

	// prometheus built from Dockerfile.
	// Check if container exists, if not, build image AND start container.
	_, exposed, _ := nat.ParsePortSpecs([]string{"9090:9090"})
	hh.Enlist("local-prometheus",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-prometheus"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-prometheus",
			ContainerConfig: &container.Config{
				Image: "testground-prometheus:latest",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyBuild,
			BuildImageOpts: &docker.BuildImageOpts{
				Name:     "testground-prometheus:latest",
				BuildCtx: filepath.Join(srcdir, "infra/docker/testground-prometheus"),
			},
		}),
	)

	// run pushgateway from downloaded image, with no additional configuraiton
	_, exposed, _ = nat.ParsePortSpecs([]string{"9091:9091"})
	hh.Enlist("local-pushgateway",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "prometheus-pushgateway"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "prometheus-pushgateway",
			ContainerConfig: &container.Config{
				Image: "prom/pushgateway",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)

	// grafana from downloaded image, with no additional configuration.
	_, exposed, _ = nat.ParsePortSpecs([]string{"3000:3000"})
	hh.Enlist("local-grafana",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-grafana"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-grafana",
			ContainerConfig: &container.Config{
				Image: "bitnami/grafana",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)

	// redis, using a downloaded image and no additional configuration.
	_, exposed, _ = nat.ParsePortSpecs([]string{"6379:6379"})
	hh.Enlist("local-redis",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-redis"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-redis",
			ContainerConfig: &container.Config{
				Image: "library/redis",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)

	// metrics exporter for redis, configured by command-line flags.
	_, exposed, _ = nat.ParsePortSpecs([]string{"1921:1921"})
	hh.Enlist("local-redis-exporter",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-redis-exporter"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-redis-exporter",
			ContainerConfig: &container.Config{
				Image: "bitnami/redis-exporter",
				Cmd:   []string{"--redis.addr", "redis://testground-redis:6379"},
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)
}

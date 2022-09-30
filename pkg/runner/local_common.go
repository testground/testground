package runner

import (
	"context"

	"github.com/docker/go-units"

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/healthcheck"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func localCommonHealthcheck(ctx context.Context, hh *healthcheck.Helper, cli *client.Client, ow *rpc.OutputWriter, controlNetworkID string, workdir string) {
	hh.Enlist("local-outputs-dir",
		healthcheck.CheckDirectoryExists(workdir),
		healthcheck.CreateDirectory(workdir),
	)

	// testground-control network
	hh.Enlist("control-network",
		healthcheck.CheckNetwork(ctx, ow, cli, controlNetworkID),
		healthcheck.CreateNetwork(ctx, ow, cli, controlNetworkID, network.IPAMConfig{Subnet: controlSubnet, Gateway: controlGateway}),
	)

	// grafana from downloaded image, with no additional configuration.
	_, exposed, _ := nat.ParsePortSpecs([]string{"3000:3000"})
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
				Cmd:   []string{"--save", "", "--appendonly", "no", "--maxclients", "120000", "--stop-writes-on-bgsave-error", "no"},
			},
			HostConfig: &container.HostConfig{
				// NOTE: we expose this port for compatibility with older sdk versions.
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
				Resources: container.Resources{
					Ulimits: []*units.Ulimit{
						{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
					},
				},
				RestartPolicy: container.RestartPolicy{
					Name: "unless-stopped",
				},
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)

	// sync service, which uses redis.
	_, exposed, _ = nat.ParsePortSpecs([]string{"5050:5050"})
	hh.Enlist("local-sync-service",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-sync-service"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-sync-service",
			ContainerConfig: &container.Config{
				Image:      "iptestground/sync-service:edge",
				Entrypoint: []string{"/service"},
				Env:        []string{"REDIS_HOST=testground-redis"},
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
				Resources: container.Resources{
					Ulimits: []*units.Ulimit{
						{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
					},
				},
				RestartPolicy: container.RestartPolicy{
					Name: "unless-stopped",
				},
			},
		}),
	)

	_, exposed, _ = nat.ParsePortSpecs([]string{"8086:8086", "8088:8088"})
	hh.Enlist("local-influxdb",
		healthcheck.CheckContainerStarted(ctx, ow, cli, "testground-influxdb"),
		healthcheck.StartContainer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-influxdb",
			ContainerConfig: &container.Config{
				Image: "library/influxdb:1.8",
				Env:   []string{"INFLUXDB_HTTP_AUTH_ENABLED=false", "INFLUXDB_DB=testground"},
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}),
	)
}

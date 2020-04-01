package runner

import (
	"context"
	"path/filepath"

	"github.com/ipfs/testground/pkg/docker"
	hc "github.com/ipfs/testground/pkg/healthcheck"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func healthcheck_common_local_infra(hcHelper *hc.HealthcheckHelper, ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, controlNetworkID string, srcdir string, workdir string) {

	// ~/.testground
	hcHelper.Enlist("local-outputs-dir",
		hc.DirExistsChecker(workdir),
		hc.DirExistsFixer(workdir),
	)

	// testground-control network
	hcHelper.Enlist(controlNetworkID,
		hc.DockerNetworkChecker(ctx, ow, cli, controlNetworkID),
		hc.DockerNetworkFixer(ctx, ow, cli, controlNetworkID, network.IPAMConfig{Subnet: controlSubnet, Gateway: controlGateway}),
	)

	// prometheus built from Dockerfile.
	// Check if container exists, if not, build image AND start container.
	_, exposed, _ := nat.ParsePortSpecs([]string{"9090:9090"})
	hcHelper.Enlist("local-prometheus",
		hc.DockerContainerChecker(ctx, ow, cli, "testground-prometheus"),
		hc.And(
			hc.DockerImageFixer(ctx, ow, cli, &docker.BuildImageOpts{
				Name:     "testground-prometheus:latest",
				BuildCtx: filepath.Join(srcdir, "infra/docker/testground-prometheus"),
			}),
			hc.DockerContainerFixer(ctx, ow, cli, &docker.EnsureContainerOpts{
				ContainerName: "testground-prometheus",
				ContainerConfig: &container.Config{
					Image: "testground-prometheus:latest",
				},
				HostConfig: &container.HostConfig{
					PortBindings: exposed,
					NetworkMode:  container.NetworkMode(controlNetworkID),
				},
				PullImageIfMissing: false,
			}),
		),
	)

	// pushgateway run from downloaded image with no additional configuraiton
	_, exposed, _ = nat.ParsePortSpecs([]string{"9091:9091"})
	hcHelper.Enlist("local-pushgateway",
		hc.DockerContainerChecker(ctx, ow, cli, "prometheus-pushgateway"),
		hc.DockerContainerFixer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "prometheus-pushgateway",
			ContainerConfig: &container.Config{
				Image: "prom/pushgateway",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			PullImageIfMissing: true,
		}),
	)

	// grafana grafana from downloaded image with no additional configuration
	_, exposed, _ = nat.ParsePortSpecs([]string{"3000:3000"})
	hcHelper.Enlist("local-grafana",
		hc.DockerContainerChecker(ctx, ow, cli, "testground-grafana"),
		hc.DockerContainerFixer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-grafana",
			ContainerConfig: &container.Config{
				Image: "bitnami/grafana",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			PullImageIfMissing: true,
		}),
	)

	// Redis, using a downloaded image and no additional configuration
	_, exposed, _ = nat.ParsePortSpecs([]string{"6379:6379"})
	hcHelper.Enlist("local-redis",
		hc.DockerContainerChecker(ctx, ow, cli, "testground-redis"),
		hc.DockerContainerFixer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-redis",
			ContainerConfig: &container.Config{
				Image: "library/redis",
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			PullImageIfMissing: true,
		}),
	)

	// metrics exporter for redis, configured by commandline flags.
	_, exposed, _ = nat.ParsePortSpecs([]string{"1921:1921"})
	hcHelper.Enlist("local-redis-exporter",
		hc.DockerContainerChecker(ctx, ow, cli, "testground-redis-exporter"),
		hc.DockerContainerFixer(ctx, ow, cli, &docker.EnsureContainerOpts{
			ContainerName: "testground-redis-exporter",
			ContainerConfig: &container.Config{
				Image: "bitnami/redis-exporter",
				Cmd:   []string{"--redis.addr", "redis://testground-redis:6379"},
			},
			HostConfig: &container.HostConfig{
				PortBindings: exposed,
				NetworkMode:  container.NetworkMode(controlNetworkID),
			},
			PullImageIfMissing: true,
		}),
	)
}

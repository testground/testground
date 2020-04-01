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
		hc.DockerNetworkFixer(ctx, ow, cli, controlNetworkID, network.IPAMConfig{}),
	)

	// prometheus built from Dockerfile.
	// Check if container exists, if not, build image AND start container.
	_, exposed, _ := nat.ParsePortSpecs([]string{"9090:9090"})
	hcHelper.Enlist("local-prometheus",
		hc.DefaultContainerChecker(ctx, ow, cli, "testground-prometheus"),
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
		hc.DefaultContainerChecker(ctx, ow, cli, "prometheus-pushgateway"),
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
}

/*
	hcHelper.Enlist("local-pushgateway",
		hc.DefaultContainerChecker(ctx,
			ow,
			cli,
			"prometheus-pushgateway"),
		hc.DefaultContainerFixer(ctx,
			ow,
			cli,
			&hc.ContainerFixerOpts{
				ContainerName: "prometheus-pushgateway",
				ImageName:     "prom/pushgateway",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"9091:9091"},
				Pull:          true,
			},
		),
	)

	// grafana
	hcHelper.Enlist("local-grafana",
		hc.DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-grafana"),
		hc.DefaultContainerFixer(ctx,
			ow,
			cli,
			&hc.ContainerFixerOpts{
				ContainerName: "testground-grafana",
				ImageName:     "bitnami/grafana",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"3000:3000"},
				Pull:          true,
			},
		),
	)

	// redis
	hcHelper.Enlist("local-redis",
		hc.DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-redis"),
		hc.DefaultContainerFixer(ctx,
			ow,
			cli,
			&hc.ContainerFixerOpts{
				ContainerName: "testground-redis",
				ImageName:     "library/redis",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"6379:6379"},
				Pull:          true,
			},
		),
	)

	// metrics for redis, customized by commandline args
	hcHelper.Enlist("local-redis-exporter",
		hc.DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-redis-exporter"),
		hc.DefaultContainerFixer(ctx,
			ow,
			cli,
			&hc.ContainerFixerOpts{
				ContainerName: "testground-redis-exporter",
				ImageName:     "bitnami/redis-exporter",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"1921:1921"},
				Pull:          true,
				Cmds: []string{
					"--redis.addr",
					"redis://testground-redis:6379",
				},
			},
		),
	)
}
*/

package runner

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/ipfs/testground/pkg/rpc"
	"path/filepath"
)

func healthcheck_common_local_infra(hcHelper HealthcheckHelper, ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, controlNetworkID string, srcdir string, workdir string) {
	// testground-control
	hcHelper.Enlist(controlNetworkID,
		DockerNetworkChecker(ctx,
			ow,
			cli,
			controlNetworkID),
		DockerNetworkFixer(ctx,
			ow,
			cli,
			controlNetworkID),
	)

	// prometheus built from Dockerfile.
	hcHelper.Enlist("local-prometheus",
		DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-prometheus"),
		CustomContainerFixer(ctx,
			ow,
			cli,
			filepath.Join(srcdir, "infra/docker/testground-prometheus"),
			&ContainerFixerOpts{
				ContainerName: "testground-prometheus",
				ImageName:     "testground-prometheus:latest",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"9090:9090"},
				Pull:          false,
			},
		))

	// pushgateway

	hcHelper.Enlist("local-pushgateway",
		DefaultContainerChecker(ctx,
			ow,
			cli,
			"prometheus-pushgateway"),
		DefaultContainerFixer(ctx,
			ow,
			cli,
			&ContainerFixerOpts{
				ContainerName: "prometheus-pushgateway",
				ImageName:     "prom/pushgateway",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"9091:9091"},
				Pull:          true,
			},
		))

	// grafana
	hcHelper.Enlist("local-grafana",
		DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-grafana"),
		DefaultContainerFixer(ctx,
			ow,
			cli,
			&ContainerFixerOpts{
				ContainerName: "testground-grafana",
				ImageName:     "bitnami/grafana",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"3000:3000"},
				Pull:          true,
			},
		))

	// redis
	hcHelper.Enlist("local-redis",
		DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-redis"),
		DefaultContainerFixer(ctx,
			ow,
			cli,
			&ContainerFixerOpts{
				ContainerName: "testground-redis",
				ImageName:     "library/redis",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"6379:6379"},
				Pull:          true,
			},
		))

	// metrics for redis, customized by commandline args
	hcHelper.Enlist("local-redis-exporter",
		DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-redis-exporter"),
		DefaultContainerFixer(ctx,
			ow,
			cli,
			&ContainerFixerOpts{
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
		))

	hcHelper.Enlist("local-outputs-dir",
		DirExistsChecker(workdir),
		DirExistsFixer(workdir),
	)
}

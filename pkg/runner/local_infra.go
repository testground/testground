package runner

import (
	"context"
	"github.com/docker/docker/api/types/container"
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
			"testground-prometheus",
			"testground-prometheus:latest",
			controlNetworkID,
			[]string{"9090:9090"},
			false,
			&container.HostConfig{},
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
			"prometheus-pushgateway",
			"prom/pushgateway",
			controlNetworkID,
			[]string{"9091:9091"},
			true,
			&container.HostConfig{},
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
			"testground-grafana",
			"bitnami/grafana",
			controlNetworkID,
			[]string{"3000:3000"},
			true,
			&container.HostConfig{},
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
			"testground-redis",
			"library/redis",
			controlNetworkID,
			[]string{"6379:6379"},
			true,
			&container.HostConfig{},
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
			"testground-redis-exporter",
			"bitnami/redis-exporter",
			controlNetworkID,
			[]string{"1921:1921"},
			true,
			&container.HostConfig{},
			"--redis.addr",
			"redis://testground-redis:6379",
		))

	hcHelper.Enlist("local-outputs-dir",
		DirExistsChecker(workdir),
		DirExistsFixer(workdir),
	)
}

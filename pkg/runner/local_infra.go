package runner

import (
	"context"
	"path/filepath"

	hc "github.com/ipfs/testground/pkg/healthcheck"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/client"
)

func healthcheck_common_local_infra(hcHelper *hc.HealthcheckHelper, ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, controlNetworkID string, srcdir string, workdir string) {

	// ~/.testground
	hcHelper.Enlist("local-outputs-dir",
		hc.DirExistsChecker(workdir),
		hc.DirExistsFixer(workdir),
	)

	// testground-control
	hcHelper.Enlist(controlNetworkID,
		hc.DockerNetworkChecker(ctx,
			ow,
			cli,
			controlNetworkID),
		hc.DockerNetworkFixer(ctx,
			ow,
			cli,
			controlNetworkID),
	)

	// prometheus built from Dockerfile.
	hcHelper.Enlist("local-prometheus",
		hc.DefaultContainerChecker(ctx,
			ow,
			cli,
			"testground-prometheus"),
		hc.CustomContainerFixer(ctx,
			ow,
			cli,
			filepath.Join(srcdir, "infra/docker/testground-prometheus"),
			&hc.ContainerFixerOpts{
				ContainerName: "testground-prometheus",
				ImageName:     "testground-prometheus:latest",
				NetworkID:     controlNetworkID,
				PortSpecs:     []string{"9090:9090"},
				Pull:          false,
			},
		),
	)

	// pushgateway
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

package generic

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	buildNetworkName = "testground-build"
)

var (
	_ api.Builder = &DockerGenericBuilder{}
)

type DockerGenericBuilder struct {
	Enabled bool
}

type DockerGenericBuilderConfig struct {
	BuildArgs map[string]*string `toml:"build_args"`
}

// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGenericBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGenericBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGenericBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		id      = in.BuildID
		basesrc = in.BaseSrcPath
		plansrc = in.TestPlanSrcPath
		sdksrc  = in.SDKSrcPath

		cli, err = client.NewClientWithOpts(cliopts...)
	)
	_ = basesrc
	_ = sdksrc

	ow = ow.With("build_id", id)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	opts := types.ImageBuildOptions{
		Tags:        []string{id, in.BuildID},
		BuildArgs:   cfg.BuildArgs,
		NetworkMode: "host",
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  plansrc,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	err = docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	ow.Infow("build completed", "took", time.Since(buildStart).Truncate(time.Second))

	out := &api.BuildOutput{
		ArtifactPath: in.BuildID,
	}

	return out, nil
}

func (*DockerGenericBuilder) ID() string {
	return "docker:generic"
}

func (*DockerGenericBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGenericBuilderConfig{})
}

package build

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	_ api.Builder = &DockerGenericBuilder{}
)

type DockerGenericBuilder struct {
	Enabled bool
}

type DockerGenericBuilderConfig struct {
	// Custom base path where we find the test source
	Path      string             `toml:"path" default:"./"`
	BuildArgs map[string]*string `toml:"build_args"` // ok if nil
}

// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGenericBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGenericBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGenericBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		basesrc  = in.UnpackedSources.BaseDir
		cli, err = client.NewClientWithOpts(cliopts...)
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	planPath := cfg.Path
	basePathForPlan := path.Join("/plan", planPath)

	if cfg.BuildArgs == nil {
		cfg.BuildArgs = make(map[string]*string)
	}

	if _, ok = cfg.BuildArgs["PLAN_PATH"]; !ok {
		cfg.BuildArgs["PLAN_PATH"] = &cfg.Path
	}

	opts := types.ImageBuildOptions{
		Tags:        []string{in.BuildID},
		BuildArgs:   cfg.BuildArgs,
		NetworkMode: "host",
		Dockerfile:  filepath.Join(basePathForPlan, "Dockerfile"),
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  basesrc,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	_, err = docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	ow.Infow("build completed", "default_tag", fmt.Sprintf("%s:latest", in.BuildID), "took", time.Since(buildStart).Truncate(time.Second))

	imageID, err := docker.GetImageID(ctx, cli, in.BuildID)
	if err != nil {
		return nil, fmt.Errorf("couldnt get docker image id: %w", err)
	}

	ow.Infow("got docker image id", "image_id", imageID)

	out := &api.BuildOutput{
		ArtifactPath: imageID,
	}

	// Testplan image tag
	testplanImageTag := fmt.Sprintf("%s:%s", in.TestPlan, imageID)

	ow.Infow("tagging image", "image_id", imageID, "tag", testplanImageTag)
	if err = cli.ImageTag(ctx, out.ArtifactPath, testplanImageTag); err != nil {
		return out, err
	}

	return out, err
}

func (*DockerGenericBuilder) ID() string {
	return "docker:generic"
}

func (*DockerGenericBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGenericBuilderConfig{})
}

func (*DockerGenericBuilder) Purge(ctx context.Context, testplan string, ow *rpc.OutputWriter) error {
	return fmt.Errorf("purge not implemented for docker:generic")
}

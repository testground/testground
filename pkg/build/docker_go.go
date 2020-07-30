package build

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-multierror"
)

const (
	DefaultBuildBaseImage = "golang:1.14.4-buster"

	buildNetworkName = "testground-build"
)

var (
	_ api.Builder      = &DockerGoBuilder{}
	_ api.Terminatable = &DockerGoBuilder{}

	dockerfileTmpl = template.Must(template.New("Dockerfile").Parse(DockerfileTemplate))
)

// DockerGoBuilder builds the test plan as a go-based container.
type DockerGoBuilder struct {
	proxyLk sync.Mutex
}

type DockerfileExtensions struct {
	PreModDownload  string `toml:"pre_mod_download"`
	PostModDownload string `toml:"post_mod_download"`
	PreSourceCopy   string `toml:"pre_source_copy"`
	PostSourceCopy  string `toml:"post_source_copy"`
	PreBuild        string `toml:"pre_build"`
	PostBuild       string `toml:"post_build"`
	PreRuntimeCopy  string `toml:"pre_runtime_copy"`
	PostRuntimeCopy string `toml:"post_runtime_copy"`
}

type DockerGoBuilderConfig struct {
	Enabled    bool
	ModulePath string `toml:"module_path"`
	ExecPkg    string `toml:"exec_pkg"`
	FreshGomod bool   `toml:"fresh_gomod"`

	// GoProxyMode specifies one of "local", "direct", "remote".
	//
	//   * The "local" mode (default) will start a proxy container (if one
	//     doesn't exist yet) with bridge networking, and will configure the
	//     build to use that proxy.
	//   * The "direct" mode sets the `GOPROXY=direct` env var on the go build.
	//   * The "remote" mode specifies a custom proxy. The `GoProxyURL` field
	//     must be non-empty.
	GoProxyMode string `toml:"go_proxy_mode"`

	// GoProxyURL specifies the URL of the proxy when GoProxyMode = "custom".
	GoProxyURL string `toml:"go_proxy_url"`

	// RuntimeImage is the runtime image that the test plan binary will be
	// copied into. Defaults to busybox:1.31.1-glibc.
	RuntimeImage string `toml:"runtime_image"`

	// BuildBaseImage is the base build image that the test plan binary will be
	// built from. Defaults to golang:1.14.4-buster
	BuildBaseImage string `toml:"build_base_image"`

	// SkipRuntimeImage allows you to skip putting the build output in a
	// slimmed-down runtime image. The build image will be emitted instead.
	SkipRuntimeImage bool `toml:"skip_runtime_image"`

	// EnableGoBuildCache enables the creation of a go build cache and its usage.
	// When enabling for the first time, a cache image will be created with the
	// dependencies of the current plan state.
	//
	// If this flag is unset or false, every build of a test plan will start
	// with a blank go container. If this flag is true, the builder will the last
	// cached image.
	EnableGoBuildCache bool `toml:"enable_go_build_cache"`

	// DockefileExtensions enables plans to inject custom Dockerfile directives.
	DockerfileExtensions DockerfileExtensions `toml:"dockerfile_extensions"`
}

type DockerfileTemplateVars struct {
	WithSDK              bool
	RuntimeImage         string
	DockerfileExtensions DockerfileExtensions
	SkipRuntimeImage     bool
}

// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGoBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGoBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	var (
		basesrc = in.UnpackedSources.BaseDir
		plansrc = in.UnpackedSources.PlanDir
		sdksrc  = in.UnpackedSources.SDKDir

		cli, err = client.NewClientWithOpts(cliopts...)
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Set up the go proxy wiring. This will start a goproxy container if
	// necessary, attaching it to the testground-build network.
	proxyURL, buildNetworkID, warn := b.setupGoProxy(ctx, ow, cli, cfg)
	if warn != nil {
		ow.Warnf("warning while setting up the go proxy: %s", warn)
	}

	// Write the Dockerfile.
	dockerfileDst := filepath.Join(basesrc, "Dockerfile")
	f, err := os.Create(dockerfileDst)
	if err != nil {
		return nil, fmt.Errorf("failed to create Dockerfile at %s: %w", dockerfileDst, err)
	}

	vars := &DockerfileTemplateVars{
		WithSDK:              sdksrc != "",
		RuntimeImage:         cfg.RuntimeImage,
		DockerfileExtensions: cfg.DockerfileExtensions,
		SkipRuntimeImage:     cfg.SkipRuntimeImage,
	}

	if err = dockerfileTmpl.Execute(f, &vars); err != nil {
		return nil, fmt.Errorf("failed to execute Dockerfile template and/or write into file %s: %w", dockerfileDst, err)
	}

	if cfg.FreshGomod {
		for _, f := range []string{"go.mod", "go.sum"} {
			file := filepath.Join(plansrc, f)
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				if err := os.Remove(file); err != nil {
					return nil, fmt.Errorf("cleanup failed; %w", err)
				}
			}
		}

		// Initialize a fresh go.mod file.
		cmd := exec.CommandContext(ctx, "go", "mod", "init", cfg.ModulePath)
		cmd.Dir = plansrc
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "creating new go.mod") {
			return nil, fmt.Errorf("unable to create go.mod; %s", out)
		}
	}

	// fall back to default build base image, if one is not configured explicitly.
	if cfg.BuildBaseImage == "" {
		cfg.BuildBaseImage = DefaultBuildBaseImage
	}

	// If we have version overrides, apply them.
	var replaces []string
	for mod, ver := range in.Dependencies {
		replaces = append(replaces, fmt.Sprintf("-replace=%s=%s@%s", mod, mod, ver))
	}

	// Inject replace directives for the SDK modules.
	if sdksrc != "" {
		replaces = append(replaces, "-replace=github.com/testground/sdk-go=../sdk")
	}

	if len(replaces) > 0 {
		// Write replace directives.
		cmd := exec.CommandContext(ctx, "go", append([]string{"mod", "edit"}, replaces...)...)
		cmd.Dir = plansrc
		if err = cmd.Run(); err != nil {
			out, _ := cmd.CombinedOutput()
			return nil, fmt.Errorf("unable to add replace directives to go.mod; %w; output: %s", err, string(out))
		}
	}

	// initial go build args.
	var args = map[string]*string{
		"GO_PROXY": &proxyURL,
	}

	if cfg.ExecPkg != "" {
		args["TESTPLAN_EXEC_PKG"] = &cfg.ExecPkg
	}
	if cfg.RuntimeImage != "" {
		args["RUNTIME_IMAGE"] = &cfg.RuntimeImage
	}

	cacheimage := fmt.Sprintf("tg-gobuildcache-%s", in.TestPlan)
	var baseimage string
	var alreadyCached bool
	if cfg.EnableGoBuildCache {
		baseimage, err = b.resolveBuildCacheImage(ctx, cli, cfg, ow, cacheimage)
		if err != nil {
			return nil, err
		}
		alreadyCached = true
	} else {
		baseimage = cfg.BuildBaseImage
	}

	args["BUILD_BASE_IMAGE"] = &baseimage

	// set BUILD_TAGS arg if the user has provided selectors.
	if len(in.Selectors) > 0 {
		s := "-tags " + strings.Join(in.Selectors, ",")
		args["BUILD_TAGS"] = &s
	}

	// Make sure we are attached to the testground-build network
	// so the builder can make use of the goproxy container.
	opts := types.ImageBuildOptions{
		Tags:        []string{in.BuildID},
		BuildArgs:   args,
		NetworkMode: "host",
	}

	// If a docker network was created for the proxy, link it to the build container
	if buildNetworkID != "" {
		opts.NetworkMode = buildNetworkName
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  basesrc,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	buildOutput, err := docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	ow.Infow("build completed", "default_tag", fmt.Sprintf("%s:latest", in.BuildID), "took", time.Since(buildStart).Truncate(time.Second))

	if cfg.EnableGoBuildCache && !alreadyCached {
		newCacheImageID := b.parseBuildCacheOutputImage(buildOutput)
		if newCacheImageID == "" {
			ow.Warnf("failed to locate go build cache output container")
		} else {
			if err := b.updateBuildCacheImage(ctx, cli, cacheimage, newCacheImageID, ow); err != nil {
				ow.Warnw("could not update build cache image tag", "error", err)
			} else {
				ow.Infow("successfully updated build cache image tag", "tag", cacheimage, "points_to", newCacheImageID)
			}
		}
	}

	imageID, err := docker.GetImageID(ctx, cli, in.BuildID)
	if err != nil {
		return nil, fmt.Errorf("couldnt get docker image id: %w", err)
	}

	ow.Infow("got docker image id", "image_id", imageID)

	deps, err := parseDependenciesFromDocker(ctx, ow, cli, imageID)
	if err != nil {
		return nil, fmt.Errorf("unable to list module dependencies; %w", err)
	}

	out := &api.BuildOutput{
		ArtifactPath: imageID,
		Dependencies: deps,
	}

	// Testplan image tag
	testplanImageTag := fmt.Sprintf("tg-plan-%s:%s", in.TestPlan, imageID)

	ow.Infow("tagging image", "image_id", imageID, "tag", testplanImageTag)
	if err = cli.ImageTag(ctx, out.ArtifactPath, testplanImageTag); err != nil {
		return out, err
	}

	return out, nil
}

func (b *DockerGoBuilder) TerminateAll(ctx context.Context, ow *rpc.OutputWriter) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// TODO: delete go proxy container and build network.
	opts := types.ImageListOptions{}
	opts.Filters = filters.NewArgs()
	opts.Filters.Add("reference", "tg-plan*")
	opts.Filters.Add("reference", "tg-gobuild*")

	images, err := cli.ImageList(ctx, opts)
	if err != nil {
		return err
	}

	var merr *multierror.Error
	for _, image := range images {
		ow.Infow("removing image", "id", image.ID, "tags", image.RepoTags)
		_, err := cli.ImageRemove(ctx, image.ID, types.ImageRemoveOptions{Force: true, PruneChildren: true})
		if err != nil {
			ow.Warnw("failed to remove image", "id", image.ID, "tags", image.RepoTags, "error", err)
		}
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

func (*DockerGoBuilder) ID() string {
	return "docker:go"
}

func (*DockerGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGoBuilderConfig{})
}

func setupLocalGoProxyVol(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client) (*mount.Mount, error) {
	volumeOpts := docker.EnsureVolumeOpts{
		Name: "testground-goproxy-vol",
	}
	vol, _, err := docker.EnsureVolume(ctx, ow.SugaredLogger, cli, &volumeOpts)
	if err != nil {
		return nil, err
	}
	mnt := mount.Mount{
		Type:   mount.TypeVolume,
		Source: vol.Name,
		Target: "/go",
	}
	return &mnt, nil
}

// setupGoProxy sets up a goproxy container, if and only if the build
// configuration requires it.
//
// If an error occurs, it is reduced to a warning, and we fall back to direct
// mode (i.e. no proxy, not even Google's default one).
func (b *DockerGoBuilder) setupGoProxy(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, cfg *DockerGoBuilderConfig) (proxyURL string, buildNetworkID string, warn error) {
	// The testground-build network is used to connect build services (like the
	// GOPROXY) to the build container.
	b.proxyLk.Lock()
	defer b.proxyLk.Unlock()

	var mnt *mount.Mount

	switch strings.TrimSpace(cfg.GoProxyMode) {
	case "direct":
		proxyURL = "direct"
		ow.Debugw("[go_proxy_mode=direct] no goproxy container will be started")

	case "remote":
		if cfg.GoProxyURL == "" {
			warn = fmt.Errorf("[go_proxy_mode=remote] no proxy URL was supplied; falling back to go_proxy_mode=direct")
			proxyURL = "direct"
			break
		}

		proxyURL = cfg.GoProxyURL
		ow.Infof("[go_proxy_mode=remote] using url: %s", proxyURL)

	case "local":
		fallthrough

	default:
		var err error
		buildNetworkID, err = docker.EnsureBridgeNetwork(ctx, ow, cli, buildNetworkName, false)
		if err != nil {
			warn = fmt.Errorf("error while creating a testground-build network: %s; forcing direct proxy mode", err)
			cfg.GoProxyMode = "direct"
			proxyURL = cfg.GoProxyURL
			break
		}

		proxyURL = "http://testground-goproxy:8081"
		mnt, warn = setupLocalGoProxyVol(ctx, ow, cli)
		if warn != nil {
			proxyURL = "direct"
			warn = fmt.Errorf("encountered an error setting up the goproxy volueme; falling back to go_proxy_mode=direct; err: %w", warn)
			break
		}
		containerOpts := docker.EnsureContainerOpts{
			ContainerName: "testground-goproxy",
			ContainerConfig: &container.Config{
				Image: "goproxy/goproxy",
			},
			HostConfig: &container.HostConfig{
				Mounts:      []mount.Mount{*mnt},
				NetworkMode: container.NetworkMode(buildNetworkID),
			},
			ImageStrategy: docker.ImageStrategyPull,
		}
		_, _, warn = docker.EnsureContainerStarted(ctx, ow, cli, &containerOpts)
		if warn != nil {
			proxyURL = "direct"
			warn = fmt.Errorf("encountered an error when creating the goproxy container; falling back to go_proxy_mode=direct; err: %w", warn)
		}
	}
	return proxyURL, buildNetworkID, warn
}

func (b *DockerGoBuilder) resolveBuildCacheImage(ctx context.Context, cli *client.Client, cfg *DockerGoBuilderConfig, ow *rpc.OutputWriter, cacheimage string) (string, error) {
	ow.Infow("go build cache enabled; checking if build cache image exists", "cache_image", cacheimage)
	_, ok, err := docker.FindImage(ctx, ow, cli, cacheimage)
	switch {
	case err != nil:
		ow.Infow("build cache image found", "cache_image", cacheimage)
		return "", err
	case ok:
		return cacheimage, nil
	}

	// We need to initialize the gobuild image for this test plan + go version.
	//  1. Check to see if the base image exists locally; if not, pull it.
	//  2. Tag the base image with `cacheimage` name.
	baseimage := cfg.BuildBaseImage

	ow.Infow("found no pre-existing build cache image; creating", "cache_image", cacheimage, "base_image", baseimage)

	switch _, ok, err := docker.FindImage(ctx, ow, cli, baseimage); {
	case err != nil:
		return "", err
	case !ok:
		ow.Infow("base image doesn't exist locally; pulling", "base_image", baseimage)
		output, err := cli.ImagePull(ctx, baseimage, types.ImagePullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull go build image: %w", err)
		}
		if _, err := docker.PipeOutput(output, ow.StdoutWriter()); err != nil {
			return "", fmt.Errorf("failed to pull go build image: %w", err)
		}
	}

	ow.Infow("tagging initial go build cache image", "cache_image", cacheimage, "base_image", baseimage)

	if err := cli.ImageTag(ctx, baseimage, cacheimage); err != nil {
		return "", fmt.Errorf("failed to tag %s as %s", baseimage, cacheimage)
	}
	return cacheimage, nil
}

func (b *DockerGoBuilder) removeBuildCacheImage(ctx context.Context, cli *client.Client, cacheimage string) error {
	// release the old tag first.
	_, err := cli.ImageRemove(ctx, cacheimage, types.ImageRemoveOptions{Force: true})
	if err != nil && !strings.Contains(err.Error(), "No such image") {
		return fmt.Errorf("failed to untag build cache image with name: %w", err)
	}
	return nil
}

func (b *DockerGoBuilder) updateBuildCacheImage(ctx context.Context, cli *client.Client, cacheimage string, newID string, _ *rpc.OutputWriter) error {
	// release the old tag first.
	err := b.removeBuildCacheImage(ctx, cli, cacheimage)
	if err != nil {
		return err
	}

	return cli.ImageTag(ctx, newID, cacheimage)
}

func (b *DockerGoBuilder) parseBuildCacheOutputImage(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	// We expect an output like:
	//
	// Step 3/28 : FROM ${BUILD_BASE_IMAGE} as builder
	// ---> 5fbd6463d24b
	// [...]
	// Step 22/30 : RUN cd ${PLAN_DIR}   && go list -m all > /testground_dep_list
	// ---> Running in eb347517d05b
	// ---> b55ef9cbbd2b
	// ---> b55ef9cbbd2b 	[[[ <==== we want to select this image ID. ]]]
	// Step 23/30 : FROM ${RUNTIME_IMAGE} AS runtime
	var foundMarker bool
	var lastLine string
	for scanner.Scan() {
		line := scanner.Text()
		if !foundMarker {
			if strings.Contains(line, "AS builder") {
				foundMarker = true
			}
			continue
		}

		if strings.Contains(line, "AS runtime") {
			// we found the end marker; select the container ID from the previous line.
			return strings.TrimPrefix(strings.TrimSpace(lastLine), "---> ")
		}
		lastLine = line
	}
	return ""
}

func (b *DockerGoBuilder) Purge(ctx context.Context, testplan string) error {
	cliopts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	cli, err := client.NewClientWithOpts(cliopts...)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err != nil {
		return err
	}

	cacheimage := fmt.Sprintf("tg-gobuildcache-%s", testplan)
	return b.removeBuildCacheImage(ctx, cli, cacheimage)
}

const DockerfileTemplate = `
# BUILD_BASE_IMAGE is the base image to use for the build. It contains a rolling
# accumulation of Go build/package caches.
ARG BUILD_BASE_IMAGE

# This Dockerfile performs a multi-stage build and RUNTIME_IMAGE is the image
# onto which to copy the resulting binary.
#
# Picking a different runtime base image from the build image allows us to
# slim down the deployable considerably.
#
# The user can override the runtime image by passing in the appropriate builder
# configuration option.
ARG RUNTIME_IMAGE=busybox:1.31.1-glibc

#:::
#::: BUILD CONTAINER
#:::
FROM ${BUILD_BASE_IMAGE} AS builder

# PLAN_DIR is the location containing the plan source inside the container.
ENV PLAN_DIR /plan

# SDK_DIR is the location containing the (optional) sdk source inside the container.
ENV SDK_DIR /sdk

# Delete any prior artifacts, if this is a cached image.
RUN rm -rf ${PLAN_DIR} ${SDK_DIR} /testground_dep_list

# TESTPLAN_EXEC_PKG is the executable package of the testplan to build.
# The image will build that package only.
ARG TESTPLAN_EXEC_PKG="."

# GO_PROXY is the go proxy that will be used, or direct by default.
ARG GO_PROXY=direct

# BUILD_TAGS is either nothing, or when expanded, it expands to "-tags <comma-separated build tags>"
ARG BUILD_TAGS

# TESTPLAN_EXEC_PKG is the executable package within this test plan we want to build. 
ENV TESTPLAN_EXEC_PKG ${TESTPLAN_EXEC_PKG}

# We explicitly set GOCACHE under the /go directory for more tidiness.
ENV GOCACHE /go/cache

{{.DockerfileExtensions.PreModDownload}}

# Copy only go.mod files and download deps, in order to leverage Docker caching.
COPY /plan/go.mod ${PLAN_DIR}/go.mod

{{if .WithSDK}}
COPY /sdk/go.mod /sdk/go.mod
{{end}}

# Download deps.
RUN echo "Using go proxy: ${GO_PROXY}" \
    && cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && go mod download

{{.DockerfileExtensions.PostModDownload}}

{{.DockerfileExtensions.PreSourceCopy}}

# Now copy the rest of the source and run the build.
COPY . /

{{.DockerfileExtensions.PostSourceCopy}}

{{.DockerfileExtensions.PreBuild}}

RUN cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && GOOS=linux GOARCH=amd64 go build -o ${PLAN_DIR}/testplan.bin ${BUILD_TAGS} ${TESTPLAN_EXEC_PKG}

{{.DockerfileExtensions.PostBuild}}

# Store module dependencies
RUN cd ${PLAN_DIR} \
  && go list -m all > /testground_dep_list

#:::
#::: (OPTIONAL) RUNTIME CONTAINER
#:::

{{ if not .SkipRuntimeImage }}

## The 'AS runtime' token is used to parse Docker stdout to extract the build image ID to cache.
FROM ${RUNTIME_IMAGE} AS runtime

# PLAN_DIR is the location containing the plan source inside the build container.
ENV PLAN_DIR /plan

{{.DockerfileExtensions.PreRuntimeCopy}}

COPY --from=builder /testground_dep_list /
COPY --from=builder ${PLAN_DIR}/testplan.bin /testplan

{{.DockerfileExtensions.PostRuntimeCopy}}

{{ else }}

## The 'AS runtime' token is used to parse Docker stdout to extract the build image ID to cache. 
FROM builder AS runtime

# PLAN_DIR is the location containing the plan source inside the build container.
ENV PLAN_DIR /plan

RUN mv ${PLAN_DIR}/testplan.bin /testplan

{{ end }}

EXPOSE 6060
ENTRYPOINT [ "/testplan"]
`

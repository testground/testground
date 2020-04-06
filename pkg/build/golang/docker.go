package golang

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/aws"
	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"

	"github.com/hashicorp/go-getter"
	"github.com/otiai10/copy"
)

var (
	_ api.Builder = &DockerGoBuilder{}
)

// DockerGoBuilder builds the test plan as a go-based container.
type DockerGoBuilder struct {
	proxyLk sync.Mutex
}

type DockerGoBuilderConfig struct {
	Enabled       bool
	GoVersion     string `toml:"go_version" overridable:"yes"`
	GoIPFSVersion string `toml:"go_ipfs_version" overridable:"yes"`
	ModulePath    string `toml:"module_path" overridable:"yes"`
	ExecPkg       string `toml:"exec_pkg" overridable:"yes"`
	FreshGomod    bool   `toml:"fresh_gomod" overridable:"yes"`

	// PushRegistry, if true, will push the resulting image to a Docker
	// registry.
	PushRegistry bool `toml:"push_registry" overridable:"yes"`

	// RegistryType is the type of registry this builder will push the generated
	// Docker image to, if PushRegistry is true.
	RegistryType string `toml:"registry_type" overridable:"yes"`

	// GoProxyMode specifies one of "on", "off", "custom".
	//
	//   * The "local" mode (default) will start a proxy container (if one
	//     doesn't exist yet) with bridge networking, and will configure the
	//     build to use that proxy.
	//   * The "direct" mode sets the `GOPROXY=direct` env var on the go build.
	//   * The "remote" mode specifies a custom proxy. The `GoProxyURL` field
	//     must be non-empty.
	GoProxyMode string `toml:"go_proxy_mode" overridable:"yes"`

	// GoProxyURL specifies the URL of the proxy when GoProxyMode = "custom".
	GoProxyURL string `toml:"go_proxy_url" overridable:"yes"`
}

// TODO cache build outputs https://github.com/ipfs/testground/issues/36
// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGoBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGoBuilderConfig, was: %T", in.BuildConfig)
	}

	cliopts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	var (
		id       = in.BuildID
		cli, err = client.NewClientWithOpts(cliopts...)
	)

	ow = ow.With("build_id", id)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// The testground-build network is used to connect build services (like the
	// GOPROXY) to the build container.
	b.proxyLk.Lock()
	//buildNetworkID, err := docker.EnsureBridgeNetwork(ctx, ow, cli, "testground-build", false)
	//if err != nil {
	//ow.Errorf("error while creating a testground-build network: %s; forcing direct proxy mode", err)
	//cfg.GoProxyMode = "direct"
	//}

	// Set up the go proxy wiring. This will start a goproxy container if
	// necessary, attaching it to the testground-build network.
	proxyURL, warn := setupGoProxy(ctx, ow, cli, buildNetworkID, cfg)
	if warn != nil {
		ow.Warnf("warning while setting up the go proxy: %s", warn)
	}
	b.proxyLk.Unlock()

	// Create a temp dir, and copy the source into it.
	tmp, err := ioutil.TempDir("", in.TestPlan.Name)
	if err != nil {
		return nil, fmt.Errorf("failed while creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	var (
		plansrc       = in.TestPlan.SourcePath
		sdksrc        = filepath.Join(in.Directories.SourceDir(), "/sdk")
		dockerfilesrc = filepath.Join(in.Directories.SourceDir(), "pkg/build/golang", "Dockerfile.template")

		plandst       = filepath.Join(tmp, "plan")
		sdkdst        = filepath.Join(tmp, "sdk")
		dockerfiledst = filepath.Join(tmp, "Dockerfile")
	)

	// Copy the plan's source; go-getter will create the dir.
	if err := getter.Get(plandst, plansrc, getter.WithContext(ctx)); err != nil {
		return nil, err
	}
	if err := materializeSymlink(plandst); err != nil {
		return nil, err
	}

	// Copy the dockerfile.
	if err := copyFile(dockerfiledst, dockerfilesrc); err != nil {
		return nil, err
	}

	// Copy the sdk source; go-getter will create the dir.
	if err := validateSdkDir(sdksrc); err != nil {
		return nil, err
	}

	if err := getter.Get(sdkdst, sdksrc, getter.WithContext(ctx)); err != nil {
		return nil, err
	}
	if err := materializeSymlink(sdkdst); err != nil {
		return nil, err
	}

	if cfg.FreshGomod {
		for _, f := range []string{"go.mod", "go.sum"} {
			file := filepath.Join(plandst, f)
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				if err := os.Remove(file); err != nil {
					return nil, fmt.Errorf("cleanup failed; %w", err)
				}
			}
		}

		// Initialize a fresh go.mod file.
		cmd := exec.CommandContext(ctx, "go", "mod", "init", cfg.ModulePath)
		cmd.Dir = plandst
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "creating new go.mod") {
			return nil, fmt.Errorf("unable to create go.mod; %s", out)
		}
	}

	// If we have version overrides, apply them.
	var replaces []string
	for mod, ver := range in.Dependencies {
		// TODO(RK): allow to override target of replaces, so we can test against forks.
		replaces = append(replaces, fmt.Sprintf("-replace=%s=%s@%s", mod, mod, ver))
	}

	// Inject replace directives for the SDK modules.
	replaces = append(replaces,
		"-replace=github.com/ipfs/testground/sdk/sync=../sdk/sync",
		"-replace=github.com/ipfs/testground/sdk/iptb=../sdk/iptb",
		"-replace=github.com/ipfs/testground/sdk/runtime=../sdk/runtime")

	// Write replace directives.
	cmd := exec.CommandContext(ctx, "go", append([]string{"mod", "edit"}, replaces...)...)
	cmd.Dir = plandst
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("unable to add replace directives to go.mod; %w", err)
	}

	// initial go build args.
	var args = map[string]*string{
		"GO_VERSION":        &cfg.GoVersion,
		"GO_IPFS_VERSION":   &cfg.GoIPFSVersion,
		"TESTPLAN_EXEC_PKG": &cfg.ExecPkg,
		"GO_PROXY":          &proxyURL,
	}

	// set BUILD_TAGS arg if the user has provided selectors.
	if len(in.Selectors) > 0 {
		s := "-tags " + strings.Join(in.Selectors, ",")
		args["BUILD_TAGS"] = &s
	}

	// Make sure we are attached to the testground-build network
	// so the builder can make use of the goproxy container.
	opts := types.ImageBuildOptions{
		Tags: []string{id, in.BuildID},
		//NetworkMode: "testground-build",
		BuildArgs: args,
	}

	imageOpts := docker.BuildImageOpts{
		BuildCtx:  tmp,
		BuildOpts: &opts,
	}

	buildStart := time.Now()

	err = docker.BuildImage(ctx, ow, cli, &imageOpts)
	if err != nil {
		return nil, err
	}

	ow.Infow("build completed", "took", time.Since(buildStart))

	deps, err := parseDependenciesFromDocker(ctx, ow, cli, in.BuildID)
	if err != nil {
		return nil, fmt.Errorf("unable to list module dependencies; %w", err)
	}

	out := &api.BuildOutput{
		ArtifactPath: in.BuildID,
		Dependencies: deps,
	}

	if cfg.PushRegistry {
		pushStart := time.Now()
		defer func() { ow.Infow("image push completed", "took", time.Since(pushStart)) }()
		if cfg.RegistryType == "aws" {
			err := pushToAWSRegistry(ctx, ow, cli, in, out)
			return out, err
		}

		if cfg.RegistryType == "dockerhub" {
			err := pushToDockerHubRegistry(ctx, ow, cli, in, out)
			return out, err
		}

		return nil, fmt.Errorf("no registry type specified, or unrecognised value: %s", cfg.RegistryType)
	}

	return out, nil
}

func (*DockerGoBuilder) ID() string {
	return "docker:go"
}

func (*DockerGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGoBuilderConfig{})
}

func pushToAWSRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.BuildInput, out *api.BuildOutput) error {
	// Get a Docker registry authentication token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(in.EnvConfig.AWS)
	if err != nil {
		return err
	}

	// AWS ECR repository name is testground-<region>-<plan_name>.
	repo := fmt.Sprintf("testground-%s-%s", in.EnvConfig.AWS.Region, in.TestPlan.Name)

	// Ensure the repo exists, or create it. Get the full URI to the repo, so we
	// can tag images.
	uri, err := aws.ECR.EnsureRepository(in.EnvConfig.AWS, repo)
	if err != nil {
		return err
	}

	// Tag the image under the AWS ECR repository.
	tag := uri + ":" + in.BuildID
	ow.Infow("tagging image", "tag", tag)
	if err = client.ImageTag(ctx, out.ArtifactPath, tag); err != nil {
		return err
	}

	// TODO for some reason, this push is way slower than the equivalent via the
	// docker CLI. Needs investigation.
	ow.Infow("pushing image", "tag", tag)
	rc, err := client.ImagePush(ctx, tag, types.ImagePushOptions{
		RegistryAuth: aws.ECR.EncodeAuthToken(auth),
	})
	if err != nil {
		return err
	}

	// Pipe the docker output to stdout.
	if err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
		return err
	}

	// replace the artifact path by the pushed image.
	out.ArtifactPath = tag
	return nil
}

func pushToDockerHubRegistry(ctx context.Context, ow *rpc.OutputWriter, client *client.Client, in *api.BuildInput, out *api.BuildOutput) error {
	uri := in.EnvConfig.DockerHub.Repo + "/testground"

	tag := uri + ":" + in.BuildID
	ow.Infow("tagging image", "source", out.ArtifactPath, "repo", uri, "tag", tag)

	if err := client.ImageTag(ctx, out.ArtifactPath, tag); err != nil {
		return err
	}

	auth := types.AuthConfig{
		Username: in.EnvConfig.DockerHub.Username,
		Password: in.EnvConfig.DockerHub.AccessToken,
	}
	authBytes, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	authBase64 := base64.URLEncoding.EncodeToString(authBytes)

	rc, err := client.ImagePush(ctx, uri, types.ImagePushOptions{
		RegistryAuth: authBase64,
	})
	if err != nil {
		return err
	}

	ow.Infow("pushed image", "source", out.ArtifactPath, "tag", tag, "repo", uri)

	// Pipe the docker output to stdout.
	if err := docker.PipeOutput(rc, ow.StdoutWriter()); err != nil {
		return err
	}

	// replace the artifact path by the pushed image.
	out.ArtifactPath = tag
	return nil
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
func setupGoProxy(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, buildNetworkID string, cfg *DockerGoBuilderConfig) (proxyURL string, warn error) {
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
				Mounts: []mount.Mount{*mnt},
				//NetworkMode: container.NetworkMode(buildNetworkID),
			},
			PullImageIfMissing: true,
		}
		_, _, warn = docker.EnsureContainer(ctx, ow, cli, &containerOpts)
		if warn != nil {
			proxyURL = "direct"
			warn = fmt.Errorf("encountered an error when creating the goproxy container; falling back to go_proxy_mode=direct; err: %w", warn)
		}
	}
	return proxyURL, warn
}

func validateSdkDir(dir string) error {
	switch fi, err := os.Stat(dir); {
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("not sdk directory: %s", dir)
	default:
		return nil
	}
}

func copyFile(dst, src string) error {
	in, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, in, 0644)
}

func materializeSymlink(dir string) error {
	if fi, err := os.Lstat(dir); err != nil {
		return err
	} else if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	// it's a symlink.
	ref, err := os.Readlink(dir)
	if err != nil {
		return err
	}
	if err := os.Remove(dir); err != nil {
		return err
	}
	return copy.Copy(ref, dir)
}

package golang

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/aws"
	"github.com/ipfs/testground/pkg/build"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	"github.com/hashicorp/go-getter"
	"github.com/otiai10/copy"
)

var (
	_ api.Builder = &DockerGoBuilder{}
)

// DockerGoBuilder builds the test plan as a go-based container.
type DockerGoBuilder struct{}

type DockerGoBuilderConfig struct {
	Enabled       bool
	GoVersion     string `toml:"go_version" overridable:"yes"`
	GoIPFSVersion string `toml:"go_ipfs_version" overridable:"yes"`
	ModulePath    string `toml:"module_path" overridable:"yes"`
	ExecPkg       string `toml:"exec_pkg" overridable:"yes"`
	FreshGomod    bool   `toml:"fresh_gomod" overridable:"yes"`
	BypassCache   bool   `toml:"bypass_cache" overridable:"yes"`

	// PushRegistry, if true, will push the resulting image to a Docker
	// registry.
	PushRegistry bool `toml:"push_registry" overridable:"yes"`

	// RegistryType is the type of registry this builder will push the generated
	// Docker image to, if PushRegistry is true.
	RegistryType string `toml:"registry_type" overridable:"yes"`
}

// TODO cache build outputs https://github.com/ipfs/testground/issues/36
// Build builds a testplan written in Go and outputs a Docker container.
func (b *DockerGoBuilder) Build(in *api.BuildInput) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*DockerGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type DockerGoBuilderConfig, was: %T", in.BuildConfig)
	}

	// TODO support specifying a docker endpoint + TLS parameters from env.
	// raulk: I don't see a need for this now, as we certainly want to do builds
	// locally, and push the image to a registry.
	// aschmahmann: No host config parameters means we need some OS check

	var host string
	if runtime.GOOS == "windows" {
		host = "npipe:////./pipe/docker_engine"
	} else {
		host = "unix:///var/run/docker.sock"
	}

	cliopts := []client.Opt{
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	}

	var (
		id          = build.CanonicalBuildID(in)
		cli, err    = client.NewClientWithOpts(cliopts...)
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	)
	defer cancel()

	if err != nil {
		return nil, err
	}

	if !cfg.BypassCache {
		// Check if an image for this build already exists.
		if exists, err := imageExists(ctx, cli, id); err != nil {
			return nil, err
		} else if exists {
			fmt.Println("found cached docker image for:", id)
			return &api.BuildOutput{ArtifactPath: id}, nil
		}
	}

	// Create a temp dir, and copy the source into it.
	tmp, err := ioutil.TempDir("", in.TestPlan.Name)
	if err != nil {
		return nil, fmt.Errorf("failed while creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	var (
		plansrc        = in.TestPlan.SourcePath
		sdksrc         = filepath.Join(in.Directories.SourceDir(), "/sdk")
		dockerfilesrc  = filepath.Join(in.Directories.SourceDir(), "pkg/build/golang", "Dockerfile.template")
		installipfssrc = filepath.Join(in.Directories.SourceDir(), "pkg", "build", "install-ipfs.sh")

		plandst        = filepath.Join(tmp, "plan")
		sdkdst         = filepath.Join(tmp, "sdk")
		dockerfiledst  = filepath.Join(tmp, "Dockerfile")
		installipfsdst = filepath.Join(tmp, "install-ipfs.sh")
	)

	// Copy the plan's source; go-getter will create the dir.
	if err := getter.Get(plandst, plansrc); err != nil {
		return nil, err
	}
	if err := materializeSymlink(plandst); err != nil {
		return nil, err
	}

	// Copy the dockerfile.
	if err := copyFile(dockerfiledst, dockerfilesrc); err != nil {
		return nil, err
	}

	// Copy the install-ipfs.sh script.
	if err := copyFile(installipfsdst, installipfssrc); err != nil {
		return nil, err
	}

	// Copy the sdk source; go-getter will create the dir.
	if err := validateSdkDir(sdksrc); err != nil {
		return nil, err
	}

	if err := getter.Get(sdkdst, sdksrc); err != nil {
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
		cmd := exec.Command("go", "mod", "init", cfg.ModulePath)
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
		fmt.Sprintf("-replace=github.com/ipfs/testground/sdk/sync=../sdk/sync"),
		fmt.Sprintf("-replace=github.com/ipfs/testground/sdk/iptb=../sdk/iptb"),
		fmt.Sprintf("-replace=github.com/ipfs/testground/sdk/runtime=../sdk/runtime"))

	// Write replace directives.
	cmd := exec.Command("go", append([]string{"mod", "edit"}, replaces...)...)
	cmd.Dir = plandst
	_, err = cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to add replace directives to go.mod; %w", err)
	}

	tar, err := archive.TarWithOptions(tmp, &archive.TarOptions{})
	if err != nil {
		return nil, err
	}

	opts := types.ImageBuildOptions{
		Tags: []string{id, in.BuildID},
		BuildArgs: map[string]*string{
			"GO_VERSION":        &cfg.GoVersion,
			"GO_IPFS_VERSION":   &cfg.GoIPFSVersion,
			"TESTPLAN_EXEC_PKG": &cfg.ExecPkg,
		},
	}

	// Build the image.
	resp, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Pipe the docker output to stdout.
	if err := util.PipeDockerOutput(resp.Body, os.Stdout); err != nil {
		return nil, err
	}

	out := &api.BuildOutput{ArtifactPath: in.BuildID}

	if cfg.PushRegistry {
		if cfg.RegistryType != "aws" {
			// We only support aws at this time.
			return nil, fmt.Errorf("no registry type specified, or unrecognised value: %s", cfg.RegistryType)
		}

		err := b.pushToAWSRegistry(ctx, cli, in, out)
		return out, err
	}

	return out, nil
}

func (b *DockerGoBuilder) pushToAWSRegistry(ctx context.Context, client *client.Client, in *api.BuildInput, out *api.BuildOutput) error {
	// Get a Docker registry authentication token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(in.EnvConfig.AWS)
	if err != nil {
		return err
	}

	// AWS ECR repository name is testground-<plan_name>.
	repo := fmt.Sprintf("testground-%s", in.TestPlan.Name)

	// Ensure the repo exists, or create it. Get the full URI to the repo, so we
	// can tag images.
	uri, err := aws.ECR.EnsureRepository(in.EnvConfig.AWS, repo)
	if err != nil {
		return err
	}

	// Tag the image under the AWS ECR repository.
	tag := uri + ":" + in.BuildID
	logging.S().Infow("tagging image", "tag", tag)
	if err = client.ImageTag(ctx, out.ArtifactPath, tag); err != nil {
		return err
	}

	// TODO for some reason, this push is way slower than the equivalent via the
	// docker CLI. Needs investigation.
	rc, err := client.ImagePush(ctx, uri, types.ImagePushOptions{
		RegistryAuth: aws.ECR.EncodeAuthToken(auth),
	})
	if err != nil {
		return err
	}

	// Pipe the docker output to stdout.
	if err := util.PipeDockerOutput(rc, os.Stdout); err != nil {
		return err
	}

	// replace the artifact path by the pushed image.
	out.ArtifactPath = tag
	return nil
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

func imageExists(ctx context.Context, cli *client.Client, id string) (bool, error) {
	summary, err := cli.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", id)),
	})
	if err != nil {
		return false, err
	}
	return len(summary) > 0, nil
}

func (*DockerGoBuilder) ID() string {
	return "docker:go"
}

func (*DockerGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(DockerGoBuilderConfig{})
}

package golang

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ipfs/testground/pkg/logging"

	"github.com/hashicorp/go-getter"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/build"
)

var (
	_ api.Builder = &ExecGoBuilder{}
)

// ExecGoBuilder (id: "exec:go") is a builder that compiles the test plan into
// an executable using the system Go SDK. The resulting artifact can be used
// with a containerless runner.
type ExecGoBuilder struct{}

type ExecGoBuilderConfig struct {
	ModulePath  string `toml:"module_path" overridable:"yes"`
	ExecPkg     string `toml:"exec_pkg" overridable:"yes"`
	FreshGomod  bool   `toml:"fresh_gomod" overridable:"yes"`
	BypassCache bool   `toml:"bypass_cache" overridable:"yes"`
}

// Build builds a testplan written in Go and outputs an executable.
func (b *ExecGoBuilder) Build(input *api.BuildInput, output io.Writer) (*api.BuildOutput, error) {
	cfg, ok := input.BuildConfig.(*ExecGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type ExecGoBuilderConfig, was: %T", input.BuildConfig)
	}

	var (
		id   = build.CanonicalBuildID(input)
		bin  = fmt.Sprintf("exec-go--%s-%s", input.TestPlan.Name, id)
		path = filepath.Join(input.Directories.WorkDir(), bin)
	)

	if !cfg.BypassCache {
		// TODO check if executable.
		if _, err := os.Stat(path); err == nil {
			return &api.BuildOutput{ArtifactPath: path}, nil
		}
	}

	// Create a temp dir, and copy the source into it.
	tmp, err := ioutil.TempDir("", input.TestPlan.Name)
	if err != nil {
		return nil, fmt.Errorf("failed while creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	var (
		plansrc = input.TestPlan.SourcePath
		sdksrc  = filepath.Join(input.Directories.SourceDir(), "/sdk")

		plandst = filepath.Join(tmp, "plan")
		sdkdst  = filepath.Join(tmp, "sdk")
	)

	// Copy the plan's source; go-getter will create the dir.
	if err := getter.Get(plandst, plansrc); err != nil {
		return nil, err
	}
	if err := materializeSymlink(plandst); err != nil {
		return nil, err
	}

	// Copy the sdk source; go-getter will create the dir.
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
	for mod, ver := range input.Dependencies {
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

	// Execute the build.
	cmd = exec.Command("go", "build", "-o", path, cfg.ExecPkg)
	cmd.Dir = plandst
	out, err := cmd.CombinedOutput()
	if err != nil {
		logging.S().Errorf("go build failed: %s", string(out))
		return nil, fmt.Errorf("failed to run the build; %w", err)
	}

	return &api.BuildOutput{ArtifactPath: path}, nil
}

func (*ExecGoBuilder) ID() string {
	return "exec:go"
}

func (*ExecGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(ExecGoBuilderConfig{})
}

package golang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/rpc"
)

var (
	_ api.Builder = &ExecGoBuilder{}
)

// ExecGoBuilder (id: "exec:go") is a builder that compiles the test plan into
// an executable using the system Go SDK. The resulting artifact can be used
// with a containerless runner.
type ExecGoBuilder struct{}

type ExecGoBuilderConfig struct {
	ModulePath string `toml:"module_path" overridable:"yes"`
	ExecPkg    string `toml:"exec_pkg" overridable:"yes"`
	FreshGomod bool   `toml:"fresh_gomod" overridable:"yes"`
}

// Build builds a testplan written in Go and outputs an executable.
func (b *ExecGoBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*ExecGoBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type ExecGoBuilderConfig, was: %T", in.BuildConfig)
	}

	var (
		id      = in.BuildID
		plansrc = in.TestPlanSrcPath
		sdksrc  = in.SDKSrcPath

		bin  = fmt.Sprintf("exec-go--%s-%s", in.TestPlan, id)
		path = filepath.Join(in.EnvConfig.Dirs().Work(), bin)
	)

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

	// If we have version overrides, apply them.
	var replaces []string
	for mod, ver := range in.Dependencies {
		replaces = append(replaces, fmt.Sprintf("-replace=%s=%s@%s", mod, mod, ver))
	}

	if sdksrc != "" {
		// Inject replace directives for the SDK modules.
		replaces = append(replaces, "-replace=github.com/testground/sdk-go=../sdk")
	}

	if len(replaces) > 0 {
		// Write replace directives.
		cmd := exec.CommandContext(ctx, "go", append([]string{"mod", "edit"}, replaces...)...)
		cmd.Dir = plansrc
		if err := cmd.Run(); err != nil {
			out, _ := cmd.CombinedOutput()
			return nil, fmt.Errorf("unable to add replace directives to go.mod; %w; output: %s", err, string(out))
		}
	}

	// Calculate the arguments to go build.
	// go build -o <output_path> [-tags <comma-separated tags>] <exec_pkg>
	var args = []string{"build", "-o", path}
	if len(in.Selectors) > 0 {
		args = append(args, "-tags")
		args = append(args, strings.Join(in.Selectors, ","))
	}
	args = append(args, cfg.ExecPkg)

	// Execute the build.
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = plansrc
	out, err := cmd.CombinedOutput()
	if err != nil {
		ow.Errorf("go build failed: %s", string(out))
		return nil, fmt.Errorf("failed to run the build; %w", err)
	}

	cmd = exec.CommandContext(ctx, "go", "list", "-m", "all")
	cmd.Dir = plansrc
	out, err = cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to list module dependencies; %w", err)
	}

	return &api.BuildOutput{
		ArtifactPath: path,
		Dependencies: parseDependencies(string(out)),
	}, nil
}

func (*ExecGoBuilder) ID() string {
	return "exec:go"
}

func (*ExecGoBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(ExecGoBuilderConfig{})
}

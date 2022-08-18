package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
)

var (
	_ api.Builder = &NixBuilder{}
)

// NixBuilder (id: "nix:generic") is a builder that build the test plan using
// Nix. The resulting artifact can be a binary or a container.
type NixBuilder struct{}

type NixBuilderConfig struct {
	// AttributeToBuild is the attribute we're building with `nix build .#<attr>`.
	// If empty, this will build the default package. e.g. `nix build .`
	AttributeToBuild string `toml:"attr"`
}

// Build builds a testplan written in Go and outputs an executable.
func (b *NixBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	cfg, ok := in.BuildConfig.(*NixBuilderConfig)
	if !ok {
		return nil, fmt.Errorf("expected configuration type NixBuilderConfig, was: %T", in.BuildConfig)
	}

	var (
		plansrc = in.UnpackedSources.PlanDir
	)

	// Execute the build.
	attrToBuild := "."
	if cfg.AttributeToBuild != "" {
		attrToBuild = attrToBuild + "#" + cfg.AttributeToBuild
	}
	cmd := exec.CommandContext(ctx, "nix", "build", attrToBuild, "--no-link", "--json", "--extra-experimental-features", "nix-command", "--extra-experimental-features", "flakes")
	cmd.Dir = plansrc
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to build: %v. %w", string(stderr.Bytes()), err)
	}

	decoder := json.NewDecoder(&stdout)
	type nixBuildOutput struct {
		DrvPath string
		Outputs map[string]string
	}
	var outputs []nixBuildOutput
	decoder.Decode(&outputs)

	return &api.BuildOutput{
		ArtifactPath: outputs[0].Outputs["out"],
	}, nil
}

func (*NixBuilder) ID() string {
	return "nix:generic"
}

func (*NixBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(NixBuilderConfig{})
}

func (*NixBuilder) Purge(ctx context.Context, testplan string, ow *rpc.OutputWriter) error {
	return fmt.Errorf("purge not implemented for nix:generic")
}

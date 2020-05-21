package build

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
)

type NullOpBuilder struct{}

type NullOpBuilderConfig struct {}

func (b *NullOpBuilder) Build(ctx context.Context, in *api.BuildInput, ow *rpc.OutputWriter) (*api.BuildOutput, error) {
	var (
		bin  = fmt.Sprintf("exec-nullop--%s", in.TestPlan)
		path = filepath.Join(in.EnvConfig.Dirs().Work(), bin)
	)

	return &api.BuildOutput{
		ArtifactPath: path,
		Dependencies: make(map[string] string),
	}, nil
}

func (*NullOpBuilder) ID() string {
	return "exec:nullop"
}

func (*NullOpBuilder) ConfigType() reflect.Type {
	return reflect.TypeOf(NullOpBuilderConfig{})
}

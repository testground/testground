package build_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/build"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/engine"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/client"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func TestBuildSelector(t *testing.T) {
	require := require.New(t)

	basedir, err := ioutil.TempDir("", "")
	require.NoError(err)

	plandir := filepath.Join(basedir, "plan")

	t.Cleanup(func() {
		os.RemoveAll(basedir)
	})

	err = copy.Copy("../../plans/placebo", plandir)
	require.NoError(err)

	env := &config.EnvConfig{}
	err = env.Load()
	require.NoError(err)

	cfg := &engine.EngineConfig{
		Builders:  []api.Builder{new(build.DockerGoBuilder), new(build.ExecGoBuilder)},
		Runners:   []api.Runner{},
		EnvConfig: env,
	}

	engine, err := engine.NewEngine(cfg)
	require.NoError(err)

	buildFn := func(builder string, selectors []string, assertion func(err error, msgsAndArgs ...interface{})) func(t *testing.T) {
		return func(t *testing.T) {
			comp := &api.Composition{
				Global: api.Global{
					Builder:        builder,
					Plan:           "placebo",
					Case:           "ok",
					TotalInstances: 1,
					BuildConfig: map[string]interface{}{
						"go_proxy_mode": "direct",
					},
				},
				Groups: []*api.Group{
					{
						ID:        "test",
						Build:     api.Build{Selectors: selectors},
						Instances: api.Instances{Count: 1},
					},
				},
			}

			unpacked := &api.UnpackedSources{
				BaseDir: basedir,
				PlanDir: plandir,
			}

			_, err = engine.DoBuild(context.TODO(), comp, unpacked, rpc.Discard())
			assertion(err)
		}
	}

	t.Run("exec:go/selectors", buildFn("exec:go", []string{"foo", "bar"}, require.Error))
	t.Run("exec:go/no_selectors", buildFn("exec:go", []string{}, require.NoError))

	// if we have a docker daemon running, test the docker runner too.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		return
	}

	t.Run("docker:go/selectors", buildFn("docker:go", []string{"foo", "bar"}, require.Error))
	t.Run("docker:go/no_selectors", buildFn("docker:go", []string{}, require.NoError))
}

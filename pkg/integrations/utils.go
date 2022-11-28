//go:build integration && docker_examples
// +build integration,docker_examples

package integrations

import (
	"context"

	"fmt"
	"testing"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"
	"github.com/urfave/cli/v2"
)

type RunSingle struct {
	plan      string
	testcase  string
	builder   string
	runner    string
	instances int
	collect   bool
	wait      bool
}

type RunResult struct {
	ExitCode int
}

func Run(t *testing.T, params RunSingle) (*RunResult, error) {
	t.Helper()

	// Create a temporary directory for the test.
	// dir, err := ioutil.TempDir("", "testground")
	// require.NoError(t, err)
	// defer os.RemoveAll(dir)

	// Start the daemon
	srv := setupDaemon(t)
	defer func() {
		srv.Shutdown(context.Background()) //nolint
	}()

	err := runHealthcheck(t, srv, params.runner)
	if err != nil {
		t.Fatal(err)
	}

	// Run the test.
	result, err := runSingle(t, params, srv)

	// Collect the results.
	// if params.collect {
	// 	result, err = Collect(t, dir, result)
	// 	require.NoError(t, err)
	// }

	return result, err
}

func setupDaemon(t *testing.T) *daemon.Daemon {
	t.Helper()

	cfg := &config.EnvConfig{
		Daemon: config.DaemonConfig{
			Scheduler: config.SchedulerConfig{
				TaskRepoType: "memory",
			},
			Listen: "localhost:0",
		},
	}
	if err := cfg.EnsureMinimalConfig(); err != nil {
		t.Fatal(err)
	}

	srv, err := daemon.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	go srv.Serve() //nolint

	return srv
}

func runImport(t *testing.T, from string, name string) error {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	args := []string{
		"testground",
		"plan",
		"import",
		"--from", from,
		"--name", name,
	}

	err := app.Run(args)

	return err
}

func runHealthcheck(t *testing.T, srv *daemon.Daemon, runner string) error {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args := []string{
		"testground",
		"--endpoint", endpoint,
		"healthcheck",
		"--runner", runner,
		"--fix",
	}

	err := app.Run(args)

	return err
}

func runSingle(t *testing.T, params RunSingle, srv *daemon.Daemon) (*RunResult, error) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args := []string{
		"testground",
		"--endpoint", endpoint,
		"run",
		"single",
		"--plan", params.plan,
		"--testcase", params.testcase,
		"--builder", params.builder,
		"--runner", params.runner,
		"--instances", fmt.Sprintf("%d", params.instances),
	}

	if params.wait {
		args = append(args, "--wait")
	}

	if params.collect {
		args = append(args, "--collect")
	}

	err := app.Run(args)

	if err != nil {
		// Known error
		if exitErr, ok := err.(cli.ExitCoder); ok {
			return &RunResult{
				ExitCode: exitErr.ExitCode(),
			}, err
		}

		// Unknown error
		return nil, err
	}

	// No error
	return &RunResult{
		ExitCode: 0,
	}, err
}

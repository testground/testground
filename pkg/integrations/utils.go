//go:build integration && docker_examples
// +build integration,docker_examples

package integrations

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"fmt"
	"testing"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"
	"github.com/urfave/cli/v2"
)

type RunSingle struct {
	plan       string
	testcase   string
	builder    string
	runner     string
	instances  int
	collect    bool
	wait       bool
	testParams []string
}

type RunResult struct {
	ExitCode int
}

func Run(t *testing.T, params RunSingle) (*RunResult, error) {
	t.Helper()

	// Create a temporary directory for the test.
	dir, err := ioutil.TempDir("", "testground")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}()

	// Change directory during the test
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Start the daemon
	srv := setupDaemon(t)
	defer func() {
		err := runTerminate(t, srv, params.runner)
		srv.Shutdown(context.Background()) //nolint

		if err != nil {
			t.Fatal(err)
		}
	}()

	err = runHealthcheck(t, srv, params.runner)
	if err != nil {
		t.Fatal(err)
	}

	// Run the test.
	result, err := runSingle(t, params, srv)
	if err != nil && result == nil {
		t.Fatal(err)
	}

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

func runTerminate(t *testing.T, srv *daemon.Daemon, runner string) error {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args := []string{
		"testground",
		"--endpoint", endpoint,
		"terminate",
		"--runner", runner,
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
	app.ExitErrHandler = func(context *cli.Context, err error) {} // Do not exit on error.

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

// use the CLI and call the command `docker pull` for each image in a list of images
func dockerPull(t *testing.T, images ...string) {
	for _, image := range images {
		t.Logf("$ docker pull %s", image)
		cmd := exec.Command("docker", "pull", image)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}

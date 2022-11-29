//go:build integration && docker_examples
// +build integration,docker_examples

package integrations

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"

	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/daemon"
	"github.com/testground/testground/pkg/task"
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
	Stdout   string
	Stderr   string
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

func makeTestgroundApp(captureExit bool) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.RootCommands
	app.Flags = cmd.RootFlags
	app.HideVersion = true

	// Capture stdout and stderr
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	app.Writer = stdout
	app.ErrWriter = stderr

	if captureExit {
		app.ExitErrHandler = func(context *cli.Context, err error) {} // Do not exit on error.
	}

	return app, stdout, stderr
}

func runImport(t *testing.T, from string, name string) error {
	t.Helper()
	app, _, _ := makeTestgroundApp(true)

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
	t.Helper()
	app, _, _ := makeTestgroundApp(true)

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
	t.Helper()
	app, _, _ := makeTestgroundApp(true)

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
	t.Helper()
	app, stdout, stderr := makeTestgroundApp(true)

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
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			}, err
		}

		// Unknown error
		return nil, err
	}

	// No error
	return &RunResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, err
}

func getMatchedGroups(regEx *regexp.Regexp, x string) map[string]string {
	match := regEx.FindStringSubmatch(x)

	if match == nil {
		return nil
	}

	group_names := regEx.SubexpNames()
	groups := make(map[string]string, len(group_names))

	for i, name := range group_names {
		if i > 0 && i <= len(match) {
			groups[name] = match[i]
		}
	}

	return groups
}

func RequireOutcomeIs(t *testing.T, outcome task.Outcome, result *RunResult) {
	t.Helper()

	// Find the string "outcome" in the result's stdout.
	// run finished with outcome = failure (single:0/1)
	match_stdout := regexp.MustCompile("run finished with outcome = (?P<outcome>[a-z0-9-]+)")
	groups := getMatchedGroups(match_stdout, result.Stdout)

	if groups == nil {
		t.Fatalf("Could not find outcome in stdout: %s", result.Stdout)
	}

	require.Equal(t, string(outcome), groups["outcome"])
}

func RequireOutcomeIsSuccess(t *testing.T, result *RunResult) {
	RequireOutcomeIs(t, task.OutcomeSuccess, result)
}

func RequireOutcomeIsFailure(t *testing.T, result *RunResult) {
	RequireOutcomeIs(t, task.OutcomeFailure, result)
}

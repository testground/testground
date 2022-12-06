//go:build integration
// +build integration

package utils

import (
	"bytes"

	"fmt"
	"testing"

	"github.com/testground/testground/pkg/cmd"
	"github.com/testground/testground/pkg/daemon"
	"github.com/urfave/cli/v2"
)

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

func runSingle(t *testing.T, params RunSingleParams, srv *daemon.Daemon) (*RunResult, error) {
	t.Helper()
	app, stdout, stderr := makeTestgroundApp(true)

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args := []string{
		"testground",
		"--endpoint", endpoint,
		"run",
		"single",
		"--plan", params.Plan,
		"--testcase", params.Testcase,
		"--builder", params.Builder,
		"--runner", params.Runner,
		"--instances", fmt.Sprintf("%d", params.Instances),
	}

	if params.Wait {
		args = append(args, "--wait")
	}

	if params.Collect {
		args = append(args, "--collect", "--collect-file", "./collected")
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

func runComposition(t *testing.T, params RunCompositionParams, srv *daemon.Daemon) (*RunResult, error) {
	t.Helper()
	app, stdout, stderr := makeTestgroundApp(true)

	endpoint := fmt.Sprintf("http://%s", srv.Addr())

	args := []string{
		"testground",
		"--endpoint", endpoint,
		"run",
		"composition",
		"--file", params.File,
	}

	if params.Wait {
		args = append(args, "--wait")
	}

	if params.Collect {
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

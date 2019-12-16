package cmd

import (
	"testing"

	"github.com/urfave/cli"
)

func TestAbortedTestShouldFailLocal(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"run",
		"placebo/abort",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
	})

	if err == nil {
		t.Fail()
	}
}

func TestAbortedTestShouldFailDocker(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"run",
		"placebo/abort",
		"--builder",
		"docker:go",
		"--runner",
		"local:docker",
	})

	if err == nil {
		t.Fail()
	}
}

func TestIncompatibleRun(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"run",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:docker",
	})

	if err == nil {
		t.Fatal("expected to get an err, due to incompatible builder and runner, but got nil")
	}
}

func TestCompatibleRun(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"run",
		"placebo/ok",
		"--builder",
		"exec:go",
		"--runner",
		"local:exec",
	})

	if err != nil {
		t.Fatal(err)
	}
}

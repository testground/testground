package cmd

import (
	"testing"

	"github.com/urfave/cli"
)

func TestAbortedTestShouldFail(t *testing.T) {
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
		t.Fail()
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
		t.Fail()
	}
}

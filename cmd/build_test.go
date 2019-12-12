package cmd

import (
	"testing"

	"github.com/urfave/cli"
)

func TestBuildExecGo(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"build",
		"placebo",
		"--builder",
		"exec:go",
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildDockerGo(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"build",
		"placebo",
		"--builder",
		"docker:go",
	})

	if err != nil {
		t.Fatal(err)
	}
}

package cmd

import (
	"testing"

	"github.com/urfave/cli"
)

func TestDescribeExistingPlan(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"describe",
		"placebo",
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestDescribeUnexistingPlan(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"describe",
		"i-do-not-exist",
	})

	if err == nil {
		t.Fatal("expected to get an err, due to non-existing test plan, but got nil")
	}
}

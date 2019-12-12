package cmd

import (
	"testing"

	"github.com/urfave/cli"
)

func TestList(t *testing.T) {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = Commands

	err := app.Run([]string{
		"testground",
		"list",
	})

	if err != nil {
		t.Fatal(err)
	}
}

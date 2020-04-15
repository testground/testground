package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/urfave/cli"

	"github.com/ipfs/testground/pkg/config"
)

// ListCommand is the specification of the `list` command.
var ListCommand = cli.Command{
	Name:   "list",
	Usage:  "list all test plans and test cases",
	Action: listCommand,
}

func listCommand(c *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	files, err := ioutil.ReadDir(cfg.Dirs().Plans())
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			fmt.Println(f.Name())
		}
	}

	return nil
}

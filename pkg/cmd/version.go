package cmd

import (
	"fmt"

	"github.com/testground/testground/pkg/version"
	"github.com/urfave/cli/v2"
)

var VersionCommand = cli.Command{
	Name:   "version",
	Usage:  "print version numbers",
	Action: versionCommand,
}

func versionCommand(c *cli.Context) error {
	fmt.Println("Testground")
	if version.GitCommit == "" {
		fmt.Println("Git commit: dirty")
		return nil
	}
	fmt.Println("Git commit:", version.GitCommit[:8])
	return nil
}

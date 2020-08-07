package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/testground/testground/pkg/config"
)

// DescribeCommand is the specification of the `describe` command.
var DescribeCommand = cli.Command{
	Name:        "describe",
	Usage:       "describe a test plan",
	Description: "Loads the test plan manifest from $TESTGROUND_HOME/plans/<plan>, and explains its contents",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "plan",
			Aliases:  []string{"p"},
			Usage:    "describe plan with name `NAME`",
			Required: true,
		},
	},
	Action: describeCommand,
}

func describeCommand(c *cli.Context) error {
	plan := c.String("plan")

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	_, manifest, err := resolveTestPlan(cfg, plan)
	if err != nil {
		return err
	}

	cases := manifest.TestCases

	manifest.Describe(os.Stdout)
	fmt.Print("TEST CASES:\n----------\n\n")

	for _, tc := range cases {
		tc.Describe(os.Stdout)
	}

	return nil
}

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/testground/testground/pkg/config"
)

// DescribeCommand is the specification of the `describe` command.
var DescribeCommand = cli.Command{
	Name:        "describe",
	Usage:       "describe a test plan",
	ArgsUsage:   "<plan name>",
	Description: "This command loads the test plan manifest from $TESTGROUND_HOME/plans/<plan name>, and explains its contents.",
	Action:      describeCommand,
}

func describeCommand(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("missing test plan location")
	}

	plan := c.Args().First()
	if strings.Contains(plan, ":") {
		return errors.New("this command expects a test plan, not a test case")
	}

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

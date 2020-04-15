package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/config"
)

// DescribeCommand is the specification of the `list` command.
var DescribeCommand = cli.Command{
	Name:      "describe",
	Usage:     "describes a test plan or test case",
	ArgsUsage: "[term], where a term is any of: <testplan> or <testplan>/<testcase>",
	Action:    describeCommand,
}

func describeCommand(c *cli.Context) error {
	if c.NArg() == 0 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("missing term to describe")
	}

	term := c.Args().First()

	var pl, tc string
	switch splt := strings.Split(term, "/"); len(splt) {
	case 2:
		pl, tc = splt[0], splt[1]
	case 1:
		pl = splt[0]
	default:
		return fmt.Errorf("unrecognized format for term")
	}

	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	_, manifest, err := resolveTestPlan(cfg, pl)
	if err != nil {
		return err
	}

	var cases []*api.TestCase
	if tc == "" {
		cases = manifest.TestCases
	} else if _, tcbn, ok := manifest.TestCaseByName(tc); ok {
		cases = []*api.TestCase{tcbn}
	} else {
		return fmt.Errorf("test case not found: %s", tc)
	}

	manifest.Describe(os.Stdout)
	fmt.Print("TEST CASES:\n----------\n----------\n")

	for _, tc := range cases {
		tc.Describe(os.Stdout)
	}

	return nil
}

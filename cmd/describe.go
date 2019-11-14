package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ipfs/testground/pkg/api"

	"github.com/urfave/cli"
)

var termExplanation = "a term is any of: <testplan> or <testplan>/<testcase>"

// DescribeCommand is the specification of the `list` command.
var DescribeCommand = cli.Command{
	Name:      "describe",
	Usage:     "describes a test plan or test case",
	ArgsUsage: "[term], where " + termExplanation,
	Action:    describeCommand,
}

func describeCommand(c *cli.Context) error {
	if c.NArg() == 0 {
		_ = cli.ShowSubcommandHelp(c)
		return errors.New("missing term to describe; " + termExplanation)
	}

	term := c.Args().First()

	var pl, tc string
	switch splt := strings.Split(term, "/"); len(splt) {
	case 2:
		pl, tc = splt[0], splt[1]
	case 1:
		pl = splt[0]
	default:
		return errors.New("unrecognized format for term;" + termExplanation)
	}

	engine, err := GetEngine()
	if err != nil {
		return err
	}
	plan := engine.TestCensus().PlanByName(pl)
	if plan == nil {
		return errors.New("plan not found")
	}

	var cases []*api.TestCase
	if tc == "" {
		cases = plan.TestCases
	} else if _, tc, ok := plan.TestCaseByName(tc); ok {
		cases = []*api.TestCase{tc}
	} else {
		return errors.New("test case not found")
	}

	plan.Describe(os.Stdout)

	fmt.Println("TESTCASES:")
	fmt.Println("----------")
	fmt.Println()

	for _, tc := range cases {
		tc.Describe(os.Stdout)
	}

	return nil
}

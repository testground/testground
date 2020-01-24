package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/ipfs/testground/pkg/sidecar"
	"github.com/urfave/cli"
)

var ErrNotLinux = fmt.Errorf("the sidecar only supports linux, not %s", runtime.GOOS)

var SidecarCommand = cli.Command{
	Name:   "sidecar",
	Usage:  "runs the sidecar daemon",
	Action: sidecarCommand,
	OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
		if runtime.GOOS != "linux" {
			return ErrNotLinux
		}
		_, _ = fmt.Fprintf(c.App.Writer, "%s %s\n\n", "Incorrect Usage.", err.Error())
		_ = cli.ShowAppHelp(c)
		return err
	},
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name:     "runner, r",
			Required: true,
			Usage:    `Specifies the runner that will be scheduling tasks that should be managed by this sidecar. Options: ` + strings.Join(sidecar.GetRunners(), ", "),
			Value: &EnumValue{
				Allowed: sidecar.GetRunners(),
			},
		},
		cli.StringFlag{
			Name:     "logs",
			Required: false,
			Usage:    `Specifies where container STDERR/STDOUT should be written. If unspecified, the sidecar doesn't save the logs`,
		},
	},
}

func sidecarCommand(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return ErrNotLinux
	}
	return sidecar.Run(c.String("runner"), c.String("logs"))
}

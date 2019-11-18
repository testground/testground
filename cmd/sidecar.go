package cmd

import (
	"strings"

	"github.com/ipfs/testground/pkg/sidecar"
	"github.com/urfave/cli"
)

var SidecarCommand = cli.Command{
	Name:   "sidecar",
	Usage:  "runs the sidecar daemon",
	Action: sidecarCommand,
	Flags: []cli.Flag{
		cli.GenericFlag{
			Name:     "runner, r",
			Required: true,
			Usage:    `Specifies the runner that will be scheduling tasks that should be managed by this sidecar. Options: ` + strings.Join(sidecar.GetRunners(), ", "),
			Value: &EnumValue{
				Allowed: sidecar.GetRunners(),
			},
		},
	},
}

func sidecarCommand(c *cli.Context) error {
	return sidecar.Run(c.String("runner"))
}

package cmd

import (
	"github.com/ipfs/testground/pkg/sidecar"
	"github.com/urfave/cli"
)

var SidecarCommand = cli.Command{
	Name:   "sidecar",
	Usage:  "runs the sidecar daemon",
	Action: sidecarCommand,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "runner, r",
			Required: true,
			Usage:    `Specifies the runner that will be scheduling tasks that should be managed by this sidecar. Options: docker`,
		},
	},
}

func sidecarCommand(c *cli.Context) error {
	return sidecar.Run(c.String("runner"))
}

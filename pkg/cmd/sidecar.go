package cmd

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/sidecar"
)

var ErrNotLinux = fmt.Errorf("the sidecar only supports linux, not %s", runtime.GOOS)

var SidecarCommand = cli.Command{
	Name:   "sidecar",
	Usage:  "run the sidecar process",
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
		&cli.StringFlag{
			Name:     "runner",
			Usage:    "runner that will be scheduling tasks that should be managed by this sidecar; supported: 'local:docker', 'cluster:k8s'",
			Required: true,
		},
	},
}

func sidecarCommand(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return ErrNotLinux
	}

	startHTTPServer()

	return sidecar.Run(c.String("runner"))
}

func startHTTPServer() {
	logging.S().Info("starting http server")
	go func() {
		_ = http.ListenAndServe(":6060", nil)
	}()
}

package main

import (
	"fmt"
	"os"

	"github.com/ipfs/testground/cmd"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "testground"
	app.Commands = cmd.Commands

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

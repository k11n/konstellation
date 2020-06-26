package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/k11n/konstellation/cmd/kon/commands"
	utilscli "github.com/k11n/konstellation/pkg/utils/cli"
	"github.com/k11n/konstellation/version"
)

func main() {
	app := cli.NewApp()
	app.Name = "kon"
	app.Usage = "Konstellation CLI. Manage Kubernetes clusters and deploy apps"
	app.EnableBashCompletion = true
	app.Version = version.Version
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name: "verbose",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("verbose") {
			utilscli.KubeDisplayOutput = true
		}
		return nil
	}
	commandSets := [][]*cli.Command{
		commands.AppCommands,
		commands.ConfigCommands,
		commands.AccountCommands,
		commands.ClusterCommands,
		commands.CertificateCommands,
		commands.LaunchCommands,
		commands.SetupCommands,
		commands.VPCCommands,
	}
	for _, cmds := range commandSets {
		app.Commands = append(app.Commands, cmds...)
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

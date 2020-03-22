package main

import (
	"fmt"
	"os"

	"github.com/davidzhao/konstellation/cmd/kon/commands"
	"github.com/davidzhao/konstellation/version"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "kon"
	app.Usage = "Konstellation CLI. Manage Kubernetes clusters and deploy apps"
	app.EnableBashCompletion = true
	app.Version = version.Version
	commandSets := [][]*cli.Command{
		commands.AppCommands,
		commands.CertificateCommands,
		commands.ConfigCommands,
		commands.ClusterCommands,
		commands.UICommands,
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

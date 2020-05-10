package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/k11n/konstellation/cmd/kon/commands"
	"github.com/k11n/konstellation/version"
)

func main() {
	app := cli.NewApp()
	app.Name = "kon"
	app.Usage = "Konstellation CLI. Manage Kubernetes clusters and deploy apps"
	app.EnableBashCompletion = true
	app.Version = version.Version
	commandSets := [][]*cli.Command{
		commands.AppCommands,
		commands.ConfigCommands,
		commands.SetupCommands,
		commands.ClusterCommands,
		commands.CertificateCommands,
		commands.DashboardCommands,
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

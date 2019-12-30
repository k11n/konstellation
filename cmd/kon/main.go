package main

import (
	"fmt"
	"os"

	"github.com/davidzhao/konstellation/cmd/kon/commands"
	"github.com/urfave/cli"
)

const (
	VERSION = "0.0.1"
)

func main() {
	app := cli.NewApp()
	app.Name = "kon"
	app.Usage = "Konstellation CLI. Manage Kubernetes clusters and deploy apps"
	app.EnableBashCompletion = true
	app.Version = VERSION
	commandSets := [][]cli.Command{
		commands.ConfigCommands,
		commands.ClusterCommands,
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

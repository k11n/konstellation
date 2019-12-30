package commands

import (
	"fmt"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/urfave/cli"
)

var ConfigCommands = []cli.Command{
	cli.Command{
		Name:   "configure",
		Usage:  "Setup Konstellation CLI",
		Action: configureStart,
	},
	// cli.Command{
	// 	Name:  "config",
	// 	Usage: "Configuration",
	// 	Subcommands: []cli.Command{
	// 		cli.Command{
	// 			Name:   "show",
	// 			Usage:  "print current config",
	// 			Action: configShow,
	// 		},
	// 	},
	// },
}

func configShow(c *cli.Context) error {
	conf := config.GetConfig()
	if conf.IsPersisted() {
		fmt.Printf("Loading config from %s\n", conf.ConfigFile())
	}
	content, err := conf.ToYAML()
	if err != nil {
		return err
	}
	fmt.Println(content)
	return nil
}

func configureStart(c *cli.Context) error {
	cloud, err := ChooseCloudPrompt("Choose a cloud provider to configure (you can use more than one)")
	if err != nil {
		return err
	}
	return cloud.Setup()
}

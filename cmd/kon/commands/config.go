package commands

import (
	"fmt"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
)

var ConfigCommands = []*cli.Command{
	&cli.Command{
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
	// install components
	installConfirmed := false
	var err error
	for _, comp := range config.Components {
		// always recheck CLI status
		if comp.NeedsCLI() {
			if !installConfirmed {
				prompt := promptui.Prompt{
					Label:     "Konstellation requires third-party tools to be installed to ~/.konstellation, ok to proceed",
					IsConfirm: true,
				}
				_, err = prompt.Run()
				if err == promptui.ErrAbort {
					fmt.Println("Configuration aborted")
					return nil
				} else if err != nil {
					return err
				}
				installConfirmed = true
			}
			fmt.Printf("Installing CLI for %s\n", comp.Name())
			err = comp.InstallCLI()
			if err != nil {
				return err
			}
		}

	}

	// cloud, err := ChooseCloudPrompt("Choose a cloud provider to configure (you can use more than one)")
	// if err != nil {
	// 	return err
	// }
	cloud := CloudAWS
	return cloud.Setup()
}

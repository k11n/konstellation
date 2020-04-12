package commands

import (
	"fmt"
	"os/exec"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
)

var ConfigCommands = []*cli.Command{
	&cli.Command{
		Name:   "setup",
		Usage:  "Setup Konstellation CLI",
		Action: setupStart,
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

var neededExes = []string{
	"aws",
	"kubectl",
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

func setupStart(c *cli.Context) error {
	if err := checkDependencies(); err != nil {
		return err
	}
	// install components
	// this only requires components that has a CLI
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

	cloud, err := ChooseCloudPrompt("Choose a cloud provider to configure")
	if err != nil {
		return err
	}
	return cloud.Setup()
}

func checkDependencies() error {
	for _, exe := range neededExes {
		_, err := exec.LookPath(exe)
		if err != nil {
			return fmt.Errorf("Konstellation requires %s, but could find it. Please ensure it's installed and in your PATH", exe)
		}
	}
	return nil
}

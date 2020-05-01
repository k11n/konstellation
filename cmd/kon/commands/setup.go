package commands

import (
	"fmt"
	"os/exec"

	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"

	"github.com/davidzhao/konstellation/cmd/kon/kube"
	"github.com/davidzhao/konstellation/pkg/components"
)

var SetupCommands = []*cli.Command{
	{
		Name:     "setup",
		Usage:    "Setup Konstellation CLI",
		Category: "Cluster",
		Action:   setupStart,
	},
}

var neededExes = []string{
	"aws",
	"kubectl",
	"terraform",
}

func setupStart(c *cli.Context) error {
	if err := checkDependencies(); err != nil {
		return err
	}
	// install components
	// this only requires components that has a CLI
	installConfirmed := false
	var err error
	for _, comp := range kube.KubeComponents {
		// always recheck CLI status
		if cliComp, ok := comp.(components.CLIComponent); ok {
			if !cliComp.NeedsCLI() {
				continue
			}
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
			err = cliComp.InstallCLI()
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

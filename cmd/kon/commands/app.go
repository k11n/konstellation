package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var AppCommands = []*cli.Command{
	&cli.Command{
		Name:  "app",
		Usage: "App management",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "list",
				Usage:  "list apps",
				Action: appList,
			},
			&cli.Command{
				Name:      "status",
				Usage:     "information about the app and its targets",
				Action:    appStatus,
				ArgsUsage: "<app>",
			},
		},
	},
}

func appList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	fmt.Printf("Listing apps on %s\n", ac.Cluster)
	apps, err := resources.ListApps(ac.kubernetesClient())
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"App", "Targets"})
	for _, app := range apps {
		targets := []string{}
		for _, target := range app.Spec.Targets {
			targets = append(targets, target.Name)
		}
		table.Append([]string{
			app.Name,
			strings.Join(targets, ","),
		})
	}
	utils.FormatTable(table)
	table.Render()
	return nil
}

func appStatus(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("app is a required argument")
	}
	appName := c.Args().Get(0)

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	app, err := resources.GetAppByName(kclient, appName)
	if err != nil {
		return err
	}
	fmt.Printf("got app: %s\n", app.Name)

	// find all the app targets
	targets, err := resources.GetAppTargets(kclient, appName)
	if err != nil {
		return err
	}

	for _, target := range targets {
		fmt.Printf("target: %s\n", target.Name)
	}
	return nil
}

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
				Name:   "status",
				Usage:  "information about the app and its targets",
				Action: appStatus,
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
	return nil
}

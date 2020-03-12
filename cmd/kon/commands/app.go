package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v2"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

var targetFlag = &cli.StringFlag{
	Name:    "target",
	Aliases: []string{"t"},
	Usage:   "filter results by target",
}

var AppCommands = []*cli.Command{
	&cli.Command{
		Name:  "app",
		Usage: "App management",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "list",
				Usage:  "list apps",
				Action: appList,
				Flags: []cli.Flag{
					targetFlag,
				},
			},
			&cli.Command{
				Name:      "status",
				Usage:     "information about the app and its targets",
				Action:    appStatus,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					targetFlag,
				},
			},
		},
	},
}

func appList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	requiredTarget := c.String("target")

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
		if requiredTarget != "" && !funk.Contains(targets, requiredTarget) {
			continue
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

	requiredTarget := c.String("target")
	// what information is useful here?
	// group by target
	// Release, date deployed, status, numAvailable/Desired, traffic
	for _, target := range targets {
		if requiredTarget != "" && target.Name != requiredTarget {
			continue
		}

		// find all targets of this app
		releases, err := resources.GetReleasesByImage(kclient,
			target.Labels[resources.RELEASE_REGISTRY_LABEL],
			target.Labels[resources.RELEASE_IMAGE_LABEL])
		if err != nil {
			return err
		}
		if len(releases) > 5 {
			releases = releases[:5] // show last 5
		}

		activeReleaseMap := map[string]*v1alpha1.AppReleaseStatus{}
		for _, ar := range target.Status.ActiveReleases {
			activeReleaseMap[ar.Release] = &ar
		}

		fmt.Printf("Target: %s\n", target.Name)
		fmt.Printf("Scale: %d-%d\n", target.Spec.Scale.Min, target.Spec.Scale.Max)
		fmt.Println()
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Release", "Date", "Pods", "Status",
		})
		for _, release := range releases {
			ar := activeReleaseMap[release.Name]
			vals := []string{
				release.ShortName(),
				release.GetCreationTimestamp().Format("2006-01-02 15:04:05"),
			}
			if ar != nil {
				vals = append(vals,
					fmt.Sprintf("%d/%d", ar.NumAvailable, ar.NumDesired),
					ar.State.String(),
				)
			} else {
				vals = append(vals,
					"",
					"archived",
				)
			}
			table.Append(vals)
		}
		table.SetBorder(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetColMinWidth(0, 30)
		table.SetColMinWidth(1, 27)
		table.SetColMinWidth(2, 8)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetHeaderLine(false)
		table.SetNoWhiteSpace(true)
		table.Render()
		fmt.Println()
	}
	return nil
}

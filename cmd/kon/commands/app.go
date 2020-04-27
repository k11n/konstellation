package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
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

var mirroredCommands = []*cli.Command{
	{
		Name:      "status",
		Usage:     "Information about the app and its targets",
		Action:    appStatus,
		ArgsUsage: "<app>",
		Flags: []cli.Flag{
			targetFlag,
		},
	},
	{
		Name:      "deploy",
		Usage:     "Deploy a new version of an app",
		Action:    appDeploy,
		ArgsUsage: "<app>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "tag",
				Usage:    "image tag to use",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "app",
				Usage:    "app to deploy",
				Required: true,
			},
		},
	},
}

var AppCommands = []*cli.Command{
	{
		Name:     "app",
		Usage:    "App management",
		Category: "App",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List apps on this cluster",
				Action: appList,
				Flags: []cli.Flag{
					targetFlag,
				},
			},
			{
				Name:   "new",
				Usage:  "Create a new app config",
				Action: appNew,
			},
			{
				Name:   "load",
				Usage:  "Load app config into Kubernetes",
				Action: appLoad,
			},
		},
	},
}

func init() {
	for _, cmd := range mirroredCommands {
		cmdCopy := *cmd
		cmdCopy.Category = "App"
		AppCommands = append(AppCommands, &cmdCopy)
	}
	AppCommands[0].Subcommands = append(AppCommands[0].Subcommands, mirroredCommands...)
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
	appName, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	// find all the app targets
	targets, err := resources.GetAppTargets(kclient, appName)
	if err != nil {
		return err
	}

	requiredTarget := c.String("target")
	// what information is useful here?
	// group by target
	// Build, date deployed, status, numAvailable/Desired, traffic
	for _, target := range targets {
		if requiredTarget != "" && target.Name != requiredTarget {
			continue
		}

		// find all targets of this app
		releases, err := resources.GetAppReleases(kclient, target.Spec.App, target.Spec.Target, 100)
		if err != nil {
			return err
		}

		// find ingress requests
		ir, err := resources.GetIngressRequestForAppTarget(kclient, target.Spec.App, target.Spec.Target)
		if err != nil {
			if err == resources.ErrNotFound {
				// just skip
			} else {
				return err
			}
		}

		fmt.Printf("\nTarget: %s\n", target.Spec.Target)
		if ir != nil {
			fmt.Printf("Hosts: %s\n", strings.Join(ir.Spec.Hosts, ", "))
			fmt.Printf("Load Balancer: %s\n", ir.Status.Address)
		}
		fmt.Printf("Scale: %d min, %d max\n", target.Spec.Scale.Min, target.Spec.Scale.Max)
		fmt.Println()
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Release", "Build", "Date", "Pods", "Status", "Traffic",
		})
		for _, release := range releases {
			// loading build
			build, err := resources.GetBuildByName(kclient, release.Spec.Build)
			if err != nil {
				return err
			}
			vals := []string{
				release.Name,
				build.ShortName(),
				release.GetCreationTimestamp().Format("2006-01-02 15:04:05"),
				fmt.Sprintf("%d/%d", release.Status.NumAvailable, release.Status.NumDesired),
				release.Status.State.String(),
				fmt.Sprintf("%d%%", release.Spec.TrafficPercentage),
			}
			table.Append(vals)
		}
		table.SetBorder(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetColMinWidth(0, 30)
		table.SetColMinWidth(1, 27)
		table.SetColMinWidth(2, 25)
		table.SetColMinWidth(3, 8)
		table.SetColMinWidth(4, 10)
		table.SetCenterSeparator(" ")
		table.SetColumnSeparator(" ")
		table.SetHeaderLine(false)
		table.SetNoWhiteSpace(true)
		table.Render()
		fmt.Println()
	}
	return nil
}

func appDeploy(c *cli.Context) error {
	appName := c.String("app")
	tag := c.String("tag")

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	targets, err := resources.GetAppTargets(kclient, appName)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return fmt.Errorf("No targets found")
	}

	appTarget := funk.Head(targets).(v1alpha1.AppTarget)
	builds, err := resources.GetBuildsByImage(kclient, appTarget.Spec.BuildRegistry, appTarget.Spec.BuildImage, 0)
	if err != nil {
		return err
	}

	// if already exists, return err
	var registry, image string
	for _, build := range builds {
		registry = build.Spec.Registry
		image = build.Spec.Image
		if build.Spec.Tag == tag {
			return fmt.Errorf("Build %s already exists", build.ShortName())
		}
	}

	if image == "" {
		return fmt.Errorf("Could not find valid build for %s", appTarget.Spec.App)
	}

	// create new build
	build := v1alpha1.NewBuild(registry, image, tag)

	_, err = resources.UpdateResource(kclient, build, nil, nil)
	return err
}

type appInfo struct {
	AppName     string
	DockerImage string
	DockerTag   string
	Target      string
}

func appNew(c *cli.Context) error {
	ai, err := promptAppInfo()
	if err != nil {
		return err
	}

	box := utils.DeployResourcesBox()
	f, err := box.Open("templates/app.yaml")
	if err != nil {
		return err
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	tmpl, err := template.New("app").Parse(string(content))
	if err != nil {
		return err
	}

	output, err := os.Create(fmt.Sprintf("%s.yaml", ai.AppName))
	if err != nil {
		return err
	}
	defer output.Close()

	err = tmpl.Execute(output, ai)
	if err != nil {
		return err
	}

	fmt.Printf("Your app config has been successfully created: %s\n", output.Name())

	return nil
}

func appLoad(c *cli.Context) error {
	return nil
}

func getAppArg(c *cli.Context) (string, error) {
	if c.NArg() == 0 {
		return "", fmt.Errorf("Required argument \"app\" was not passed in")
	}
	return c.Args().Get(0), nil
}

func promptAppInfo() (ai *appInfo, err error) {
	prompt := promptui.Prompt{
		Label:    "Enter app name",
		Validate: utils.ValidateKubeName,
	}
	utils.FixPromptBell(&prompt)

	ai = &appInfo{}
	val, err := prompt.Run()
	if err != nil {
		return
	}
	ai.AppName = val

	// docker image
	prompt = promptui.Prompt{
		Label:    "Docker image",
		Validate: utils.ValidateMinLength(3),
	}
	utils.FixPromptBell(&prompt)
	if val, err = prompt.Run(); err != nil {
		return
	}

	// see if tag is provided
	parts := strings.Split(val, ":")
	if len(parts) == 2 {
		ai.DockerImage = parts[0]
		ai.DockerTag = parts[1]
	} else {
		ai.DockerImage = val
		ai.DockerTag = "latest"
	}

	// if connected to kube, load the first target
	ac, err := getActiveCluster()
	if err != nil {
		return ai, nil
	}

	cc, err := resources.GetClusterConfig(ac.kubernetesClient())
	if err != nil {
		return ai, nil
	}

	if len(cc.Spec.Targets) > 0 {
		ai.Target = cc.Spec.Targets[0]
	}
	return
}

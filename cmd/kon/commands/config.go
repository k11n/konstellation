package commands

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
	utilcli "github.com/k11n/konstellation/pkg/utils/cli"
)

// app configs

var (
	appFilterFlag = &cli.StringFlag{
		Name:    "app",
		Aliases: []string{"a"},
		Usage:   "filter results by app",
	}
	nameFlag = &cli.StringFlag{
		Name:  "name",
		Usage: "name of a shared config (must pass in either --name or --app)",
	}
	appFlag = &cli.StringFlag{
		Name:  "app",
		Usage: "app name (must pass in either --name or --app)",
	}
)

var ConfigCommands = []*cli.Command{
	{
		Name:     "config",
		Usage:    "Config for apps",
		Category: "App",
		Subcommands: []*cli.Command{
			{
				Name:   "delete",
				Usage:  "Delete a config",
				Action: configDelete,
				Flags: []cli.Flag{
					nameFlag,
					appFlag,
					&cli.StringFlag{
						Name:  "target",
						Usage: "delete config for a single target",
					},
				},
			},
			{
				Name:   "edit",
				Usage:  "Create or edit a config. Use --app to edit an app config, or --name to edit a shared config",
				Action: configEdit,
				Flags: []cli.Flag{
					nameFlag,
					appFlag,
					&cli.StringFlag{
						Name:  "target",
						Usage: "edit config only for a specific target (target values will override the base config)",
					},
				},
			},
			{
				Name:   "list",
				Usage:  "List config files on this cluster",
				Action: configList,
				Flags: []cli.Flag{
					appFilterFlag,
				},
			},
			{
				Name:      "show",
				Usage:     "Show config for a release of the app",
				Action:    configShow,
				ArgsUsage: "<release>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "env",
						Usage: "when set, displays the environment variables that your app would receive",
					},
				},
			},
		},
	},
}

func configList(c *cli.Context) error {
	labels := client.MatchingLabels{}
	app := c.String("app")
	if app != "" {
		labels[resources.AppLabel] = app
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"Type",
		"App",
		"Name (shared config)",
		"Target",
	})
	resources.ForEach(kclient, &v1alpha1.AppConfigList{}, func(item interface{}) error {
		ac := item.(v1alpha1.AppConfig)
		app := ""
		name := ""

		if ac.Type == v1alpha1.ConfigTypeShared {
			name = ac.GetSharedName()
		} else {
			app = ac.GetAppName()
		}

		table.Append([]string{
			string(ac.Type),
			app,
			name,
			ac.Labels[v1alpha1.TargetLabel],
		})
		return nil
	}, labels)

	utils.FormatTable(table)
	table.Render()

	return nil
}

func configShow(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("Required argument <release> was not passed in")
	}
	release := c.Args().Get(0)

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	showEnv := c.Bool("env")

	kclient := ac.kubernetesClient()
	// find valid targets for the app
	ar, err := resources.GetAppReleaseByName(kclient, release, "")
	if err != nil {
		return err
	}

	if ar.Spec.Config == "" {
		return fmt.Errorf("Release %s does not have a config", release)
	}

	cm, err := resources.GetConfigMap(kclient, resources.NamespaceForAppTarget(ar.Spec.App, ar.Spec.Target), ar.Spec.Config)
	if err != nil {
		return err
	}

	if showEnv {
		keys := funk.Keys(cm.Data).([]string)
		sort.Strings(keys)
		for _, key := range keys {
			if key == v1alpha1.ConfigFileName {
				continue
			}
			fmt.Printf("%s=%s\n", key, cm.Data[key])
		}
	} else {
		fmt.Println(cm.Data[v1alpha1.ConfigFileName])
	}

	return nil
}

func configEdit(c *cli.Context) error {
	confType, name, err := getAppOrShared(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	target := c.String("target")
	kclient := ac.kubernetesClient()

	appConfig, err := resources.GetConfigForType(kclient, confType, name, target)
	if err == resources.ErrNotFound {
		if confType == v1alpha1.ConfigTypeApp {
			appConfig = v1alpha1.NewAppConfig(name, target)
		} else {
			appConfig = v1alpha1.NewSharedConfig(name, target)
		}
	} else if err != nil {
		return err
	}

	// launch editor
	data, err := utilcli.ExecuteUserEditor(appConfig.ConfigYaml, fmt.Sprintf("%s.yaml", appConfig.Name))
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return fmt.Errorf("config not saved, file is empty")
	}

	// persist
	appConfig.ConfigYaml = data
	err = resources.SaveAppConfig(kclient, appConfig)
	if err != nil {
		return err
	}

	targetStr := ""
	if target != "" {
		targetStr = fmt.Sprintf(", target %s", target)
	}
	fmt.Printf("Saved %s config for %s%s.\n", confType, name, targetStr)
	return nil
}

func configDelete(c *cli.Context) error {
	confType, name, err := getAppOrShared(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	target := c.String("target")
	kclient := ac.kubernetesClient()
	appConfig, err := resources.GetConfigForType(kclient, confType, name, target)
	if err == resources.ErrNotFound {
		return fmt.Errorf("Config does not exist")
	} else if err != nil {
		return err
	}

	err = kclient.Delete(context.TODO(), appConfig)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %s config: %s.\n", confType, name)

	return nil
}

func getAppOrShared(c *cli.Context) (t v1alpha1.ConfigType, n string, err error) {
	app := c.String("app")
	name := c.String("name")

	if app == "" && name == "" {
		err = fmt.Errorf("Either --app or --name is required")
		return
	}
	if app != "" && name != "" {
		err = fmt.Errorf("Both --app and --name cannot be used at the same time")
		return
	}

	if app != "" {
		t = v1alpha1.ConfigTypeApp
		n = app
	} else {
		t = v1alpha1.ConfigTypeShared
		n = name
	}

	err = utils.ValidateKubeName(n)
	return
}

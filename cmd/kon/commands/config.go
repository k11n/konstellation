package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
	utilcli "github.com/davidzhao/konstellation/pkg/utils/cli"
)

// app configs

var (
	appFilterFlag = &cli.StringFlag{
		Name:    "app",
		Aliases: []string{"a"},
		Usage:   "filter results by app",
	}
)

var ConfigCommands = []*cli.Command{
	{
		Name:     "config",
		Usage:    "Config for apps",
		Category: "App",
		Subcommands: []*cli.Command{
			{
				Name:      "delete",
				Usage:     "Delete config for an app",
				Action:    configDelete,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "target",
						Usage: "delete only this target's config",
					},
				},
			},
			{
				Name:      "edit",
				Usage:     "Edit an app config, creating it if it doesn't exist",
				Action:    configEdit,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "target",
						Usage: "edit config only for a specific target (merged with app config)",
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
				Usage:     "Show config for an app",
				Action:    configShow,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "target",
						Usage: "when target is set, shows the merged config with target overrides",
					},
				},
			},
		},
	},
}

func configList(c *cli.Context) error {
	return nil
}

func configShow(c *cli.Context) error {
	app, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	target := c.String("target")

	kclient := ac.kubernetesClient()
	appConfig, err := resources.GetAppConfig(kclient, app, target)
	if err == resources.ErrNotFound {
		return fmt.Errorf("Config does not exist")
	} else if err != nil {
		return err
	}

	fmt.Println(string(appConfig.ConfigYaml))

	return nil
}

func configEdit(c *cli.Context) error {
	app, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	target := c.String("target")
	kclient := ac.kubernetesClient()

	_, err = resources.GetAppByName(kclient, app)
	if err != nil {
		return err
	}

	appConfig, err := resources.GetAppConfig(kclient, app, target)
	if err == resources.ErrNotFound {
		appConfig = v1alpha1.NewAppConfig(app, target)
	} else if err != nil {
		return err
	}

	// launch editor
	data, err := utilcli.ExecuteUserEditor(appConfig.ConfigYaml, fmt.Sprintf("%s.yaml", appConfig.Name))
	if err != nil {
		return err
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
	fmt.Printf("Saved config for %s%s.\n", app, targetStr)
	return nil
}

func configDelete(c *cli.Context) error {
	app, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	target := c.String("target")
	kclient := ac.kubernetesClient()
	appConfig, err := resources.GetAppConfig(kclient, app, target)
	if err == resources.ErrNotFound {
		return fmt.Errorf("Config does not exist")
	} else if err != nil {
		return err
	}

	err = kclient.Delete(context.TODO(), appConfig)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted app config")

	return nil
}

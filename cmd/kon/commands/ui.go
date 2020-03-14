package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/davidzhao/konstellation/pkg/components/kubedash"
	koncli "github.com/davidzhao/konstellation/pkg/utils/cli"
)

var UICommands = []*cli.Command{
	{
		Name:  "ui",
		Usage: "Launch various WebUI",
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
		Subcommands: []*cli.Command{
			{
				Name:   "kube",
				Usage:  "Launch Kubernetes Dashboard",
				Action: kubeDashboard,
			},
		},
	},
}

func kubeDashboard(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	// print token
	token, err := ac.Cloud.KubernetesProvider().GetAuthToken(context.TODO(), ac.Cluster)
	if err != nil {
		return errors.Wrap(err, "failed to get authentication token")
	}

	fmt.Printf("Authentication token (copy and use token auth)\n%s\n\n", token.Status.Token)

	// run proxy
	proxy := koncli.NewKubeProxy()
	err = proxy.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start kubernetes proxy")
	}
	fmt.Printf("Starting kube proxy at %s\n", proxy.HostWithPort())

	// launch web browser after delay
	time.Sleep(3 * time.Second)
	browser.OpenURL(proxy.HostWithPort() + kubedash.PROXY_PATH)

	proxy.WaitUntilDone()
	return nil
}

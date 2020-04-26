package commands

import (
	"fmt"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/davidzhao/konstellation/pkg/components/kubedash"
	"github.com/davidzhao/konstellation/pkg/resources"
	koncli "github.com/davidzhao/konstellation/pkg/utils/cli"
)

var UICommands = []*cli.Command{
	{
		Name:     "ui",
		Usage:    "Launch various WebUI",
		Category: "Cluster",
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
	secret, err := resources.GetSecretForAccount(ac.kubernetesClient(), resources.SERVICE_ACCOUNT_KON_ADMIN)
	//token, err := ac.Manager.KubernetesProvider().GetAuthToken(context.TODO(), ac.Cluster)
	if err != nil {
		return errors.Wrap(err, "failed to get authentication token")
	}

	//fmt.Printf("Authentication token (copy and use token auth)\n%s\n\n", token.Status.Token)
	fmt.Printf("Authentication token (copy and use token auth)\n%s\n\n", secret.Data["token"])

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

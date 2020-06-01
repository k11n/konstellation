package commands

import (
	"fmt"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/k11n/konstellation/pkg/components/kubedash"
	"github.com/k11n/konstellation/pkg/resources"
	koncli "github.com/k11n/konstellation/pkg/utils/cli"
)

var DashboardCommands = []*cli.Command{
	{
		Name:     "dashboard",
		Usage:    "Launch various dashboards",
		Category: "Cluster",
		Before: func(c *cli.Context) error {
			return ensureClusterSelected()
		},
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
		Subcommands: []*cli.Command{
			{
				Name:   "kube",
				Usage:  "Launch Kubernetes Dashboard",
				Action: kubeDashboard,
			},
			{
				Name:   "kiali",
				Usage:  "Launch Kiali (mesh)",
				Action: kialiDashboard,
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
	secret, err := resources.GetSecretForAccount(ac.kubernetesClient(), resources.KonSystemNamespace, resources.SERVICE_ACCOUNT_KON_ADMIN)
	if err != nil {
		return errors.Wrap(err, "failed to get authentication token")
	}

	fmt.Printf("Authentication token (copy and use token auth)\n%s\n\n", secret.Data["token"])

	// run proxy
	proxy := koncli.NewKubeProxy()
	err = proxy.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start kubernetes proxy")
	}

	// launch web browser after delay
	fmt.Printf("Launching Kubernetes Dashboard: %s\n", proxy.URL())

	time.Sleep(2 * time.Second)
	browser.OpenURL(proxy.URL() + kubedash.ProxyPath)

	proxy.WaitUntilDone()
	return nil
}

func kialiDashboard(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	// print token
	secret, err := resources.GetSecret(ac.kubernetesClient(), resources.IstioNamespace, "kiali")
	if err != nil {
		return errors.Wrap(err, "failed to get authentication token")
	}

	fmt.Printf("Username: %s\n", secret.Data["username"])
	fmt.Printf("Passphrase: %s\n\n", secret.Data["passphrase"])

	// run proxy
	proxy, err := koncli.NewKubeProxyForService(ac.kubernetesClient(), "istio-system", "kiali", 0)
	if err != nil {
		return err
	}
	err = proxy.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start kubernetes proxy")
	}
	fmt.Printf("Launching Kiali Dashboard: %s\n", proxy.URL())

	// launch web browser after delay
	time.Sleep(2 * time.Second)
	browser.OpenURL(proxy.URL())

	proxy.WaitUntilDone()
	return nil
}

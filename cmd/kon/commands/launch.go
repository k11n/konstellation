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

var LaunchCommands = []*cli.Command{
	{
		Name:     "launch",
		Usage:    "Launch management webapps",
		Category: "Cluster",
		Before: func(c *cli.Context) error {
			return ensureClusterSelected()
		},
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
		Subcommands: []*cli.Command{
			{
				Name:   "alertmanager",
				Usage:  "Launch Alert Manager",
				Action: launchAlertManager,
			},
			{
				Name:   "grafana",
				Usage:  "Launch Grafana",
				Action: launchGrafana,
			},
			{
				Name:   "kubedash",
				Usage:  "Launch Kubernetes Dashboard",
				Action: launchKubeDash,
			},
			{
				Name:   "prometheus",
				Usage:  "Launch Prometheus UI",
				Action: launchPrometheus,
			},
		},
	},
}

func launchKubeDash(c *cli.Context) error {
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
	url := proxy.URL() + kubedash.ProxyPath
	fmt.Printf("Launching Kubernetes Dashboard: %s\n", url)

	time.Sleep(2 * time.Second)
	browser.OpenURL(url)

	proxy.WaitUntilDone()
	return nil
}

func launchPrometheus(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	proxy, err := koncli.NewKubeProxyForService(ac.kubernetesClient(), resources.KonSystemNamespace, "prometheus-k8s", 9090)
	if err != nil {
		return err
	}
	return startProxyAndWait(proxy, "Prometheus")
}

func launchGrafana(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	proxy, err := koncli.NewKubeProxyForService(ac.kubernetesClient(), resources.GrafanaNamespace, "grafana-service", 3000)
	if err != nil {
		return err
	}
	return startProxyAndWait(proxy, "Grafana")
}

func launchAlertManager(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	proxy, err := koncli.NewKubeProxyForService(ac.kubernetesClient(), resources.KonSystemNamespace, "alertmanager-main", 9093)
	if err != nil {
		return err
	}
	return startProxyAndWait(proxy, "Alert Manager")
}

func startProxyAndWait(proxy *koncli.KubeProxy, name string) error {
	err := proxy.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start kubernetes proxy")
	}
	if name != "" {
		fmt.Printf("Launching %s: %s\n", name, proxy.URL())
	}

	// launch web browser after delay
	time.Sleep(2 * time.Second)
	browser.OpenURL(proxy.URL())

	proxy.WaitUntilDone()
	return nil
}

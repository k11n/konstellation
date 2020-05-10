package ingress

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/utils/cli"
)

type NginxIngress struct {
}

func (i *NginxIngress) Name() string {
	return "ingress.nginx"
}

func (i *NginxIngress) Version() string {
	return "0.30.0"
}

func (i *NginxIngress) InstallComponent(kclient client.Client) error {
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes/ingress-nginx/nginx-%s/deploy/static/mandatory.yaml",
		i.Version())
	return cli.KubeApply(url)
}

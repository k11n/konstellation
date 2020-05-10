package kubedash

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

const (
	ProxyPath = "/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"
)

func init() {
	components.RegisterComponent(&KubeDash{})
}

type KubeDash struct {
}

func (d *KubeDash) Name() string {
	return "kube.dashboard"
}

func (d *KubeDash) Version() string {
	// TODO: should we match Kube versions
	return "2.0.0"
}

func (d *KubeDash) InstallComponent(kclient client.Client) error {
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes/dashboard/v%s/aio/deploy/recommended.yaml",
		d.Version())
	return cli.KubeApply(url)
}

package kubedash

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

const (
	ProxyPath = "/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy/"
)

func init() {
	components.RegisterComponent(&KubeDash{})
}

type KubeDash struct {
}

func (d *KubeDash) Name() string {
	return "kube.dashboard"
}

func (d *KubeDash) VersionForKube(version string) string {
	return "2.0.5"
}

func (d *KubeDash) InstallComponent(kclient client.Client) error {
	return cli.KubeApplyFromBox("kube-dashboard.yaml", "")
}

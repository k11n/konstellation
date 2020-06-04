package kubedash

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/components"
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

func (d *KubeDash) Version() string {
	// TODO: should we match Kube versions
	return "2.0.0"
}

func (d *KubeDash) InstallComponent(kclient client.Client) error {
	return utils.KubeApplyFile("kube-dashboard.yaml", "")
}

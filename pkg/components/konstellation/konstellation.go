package konstellation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/utils/cli"
	"github.com/k11n/konstellation/version"
)

type Konstellation struct {
}

func (k *Konstellation) Name() string {
	return "konstellation"
}

func (k *Konstellation) VersionForKube(kubeVersion string) string {
	return version.Version
}

func (k *Konstellation) InstallComponent(client.Client) error {
	return cli.KubeApplyFromBox("operator.yaml", "")
}

package grafana

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/components"
)

const grafanaVersion = "3.4.0"

func init() {
	components.RegisterComponent(&GrafanaOperator{})
}

type GrafanaOperator struct {
}

func (d *GrafanaOperator) Name() string {
	return "grafana-operator"
}

func (d *GrafanaOperator) VersionForKube(version string) string {
	return grafanaVersion
}

func (d *GrafanaOperator) InstallComponent(kclient client.Client) error {
	err := utils.Retry(func() error {
		return utils.KubeApplyFile("grafana/operator.yaml", "")
	}, 8, 0)
	if err != nil {
		return err
	}

	err = utils.Retry(func() error {
		return utils.KubeApplyFile("grafana/dashboards.yaml", "")
	}, 8, 0)
	if err != nil {
		return err
	}

	// TODO: create grafana dashboards
	return nil
}

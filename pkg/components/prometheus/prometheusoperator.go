package prometheus

import (
	"fmt"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/utils/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	components.RegisterComponent(&PrometheusOperator{})
}

type PrometheusOperator struct {
}

func (d *PrometheusOperator) Name() string {
	return "prometheus-operator"
}

func (d *PrometheusOperator) Version() string {
	return "0.38"
}

func (d *PrometheusOperator) InstallComponent(kclient client.Client) error {
	url := fmt.Sprintf("https://raw.githubusercontent.com/coreos/prometheus-operator/release-%s/bundle.yaml",
		d.Version())
	return cli.KubeApply(url)
}

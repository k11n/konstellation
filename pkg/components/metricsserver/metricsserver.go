package metricsserver

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/components"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

func init() {
	components.RegisterComponent(&MetricsServer{})
}

type MetricsServer struct {
}

func (m *MetricsServer) Name() string {
	return "metrics-server"
}

func (m *MetricsServer) Version() string {
	return "0.3.6"
}

// installs the component onto the kube cluster
func (m *MetricsServer) InstallComponent(kclient client.Client) error {
	u := fmt.Sprintf("https://github.com/kubernetes-sigs/metrics-server/releases/download/v%s/components.yaml",
		m.Version())

	return cli.KubeApply(u)
}

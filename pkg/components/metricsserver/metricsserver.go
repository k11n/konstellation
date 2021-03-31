package metricsserver

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

func init() {
	components.RegisterComponent(&MetricsServer{})
}

const (
	componentVersion = "0.4.2"
)

type MetricsServer struct {
}

func (m *MetricsServer) Name() string {
	return "metrics-server"
}

func (m *MetricsServer) VersionForKube(version string) string {
	return componentVersion
}

// installs the component onto the kube cluster
func (m *MetricsServer) InstallComponent(kclient client.Client) error {
	u := fmt.Sprintf("https://github.com/kubernetes-sigs/metrics-server/releases/download/v%s/components.yaml",
		componentVersion)

	return cli.KubeApply(u)
}

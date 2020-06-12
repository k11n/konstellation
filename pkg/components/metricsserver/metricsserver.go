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
	version = "0.3.6"
)

type MetricsServer struct {
}

func (m *MetricsServer) Name() string {
	return "metrics-server"
}

func (m *MetricsServer) VersionForKube(version string) string {
	return version
}

// installs the component onto the kube cluster
func (m *MetricsServer) InstallComponent(kclient client.Client) error {
	u := fmt.Sprintf("https://github.com/kubernetes-sigs/metrics-server/releases/download/v%s/components.yaml",
		version)

	return cli.KubeApply(u)
}

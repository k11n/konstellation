package prometheus

import (
	"fmt"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
	"github.com/k11n/konstellation/pkg/utils/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	versionMap = map[string]string{
		"1.16": "0.4",
		"1.17": "0.4",
		"1.18": "0.6",
	}
)

const (
	ComponentName   = "kube-prometheus"
	DiskSizeKey     = "disk-size"
	DefaultDiskSize = "100Gi"
)

func init() {
	components.RegisterComponent(&KubePrometheus{})
}

type KubePrometheus struct {
}

func (d *KubePrometheus) Name() string {
	return ComponentName
}

func (d *KubePrometheus) VersionForKube(version string) string {
	return versionMap[version]
}

func (d *KubePrometheus) InstallComponent(kclient client.Client) error {
	// get cluster config and alb service account to annotate
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	version := d.VersionForKube(cc.Spec.KubeVersion)

	err = retry.Retry(func() error {
		return cli.KubeApplyFromBox(fmt.Sprintf("kube-prometheus/%s/prometheus-operator.yaml", version), "")
	}, 8, 0)
	if err != nil {
		return err
	}

	err = retry.Retry(func() error {
		return cli.KubeApplyFromBox(fmt.Sprintf("kube-prometheus/%s/prometheus-k8s.yaml", version), "")
	}, 8, 0)
	return err
}

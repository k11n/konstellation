package autoscaler

import (
	"bytes"
	"io/ioutil"
	"text/template"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/assets"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

func init() {
	components.RegisterComponent(&ClusterAutoScaler{})
}

var (
	versionMap = map[string]string{
		"1.17": "1.17.4",
		"1.18": "1.18.3",
		"1.19": "1.19.1",
	}
)

type ClusterAutoScaler struct {
}

func (s *ClusterAutoScaler) Name() string {
	return "cluster-autoscaler"
}
func (s *ClusterAutoScaler) VersionForKube(version string) string {
	return versionMap[version]
}

type autoScalerConfig struct {
	ClusterName string
	Version     string
}

func (s *ClusterAutoScaler) InstallComponent(kclient client.Client) error {
	// grab config
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	box := assets.DeployResourcesBox()
	f, err := box.Open("templates/cluster-autoscaler.yaml")
	if err != nil {
		return err
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	tmpl, err := template.New("autoscaler").Parse(string(content))
	if err != nil {
		return err
	}

	conf := autoScalerConfig{
		ClusterName: cc.Name,
		Version:     s.VersionForKube(cc.Spec.KubeVersion),
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, conf)
	if err != nil {
		return err
	}

	return cli.KubeApplyReader(buf)
}

package autoscaler

import (
	"bytes"
	"io/ioutil"
	"text/template"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

func init() {
	components.RegisterComponent(&ClusterAutoScaler{})
}

type ClusterAutoScaler struct {
}

func (s *ClusterAutoScaler) Name() string {
	return "cluster-autoscaler"
}
func (s *ClusterAutoScaler) Version() string {
	return "1.18"
}

type autoScalerConfig struct {
	ClusterName string
}

func (s *ClusterAutoScaler) InstallComponent(kclient client.Client) error {
	// grab config
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	box := utils.DeployResourcesBox()
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
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, conf)
	if err != nil {
		return err
	}

	return cli.KubeApplyReader(buf)
}

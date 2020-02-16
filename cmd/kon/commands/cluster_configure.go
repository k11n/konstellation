package commands

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/thoas/go-funk"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

func (c *activeCluster) configureCluster() error {
	fmt.Printf("Configuring %s (%s)\n\n", c.Cluster, c.Cloud.ID())
	kclient := c.kubernetesClient()
	// pull existing config
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	actions := []string{
		"Add target",
		"Remove target",
	}

	s := promptui.Select{
		Label: "Configure action",
		Items: actions,
	}
	idx, _, err := s.Run()
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		err = c.addTargetPrompt(cc)
	}
	return err
}

func (c *activeCluster) addTargetPrompt(cc *v1alpha1.ClusterConfig) error {
	prompt := promptui.Prompt{
		Label: "Target name",
		// TODO: validate
	}

	val, err := prompt.Run()
	if err != nil {
		return err
	}

	if funk.Contains(cc.Spec.Targets, val) {
		return fmt.Errorf("Target %s already exists on cluster %s", val, c.Cluster)
	}
	cc.Spec.Targets = append(cc.Spec.Targets, val)
	resources.UpdateClusterConfig(c.kubernetesClient(), cc)
	return nil
}

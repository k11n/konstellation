package commands

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/thoas/go-funk"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

func (c *activeCluster) configureCluster() error {
	fmt.Printf("Configuring cluster %s (%s)\n\n", c.Cluster)
	kclient := c.kubernetesClient()
	// pull existing config
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	if len(cc.Spec.Targets) == 0 {
		fmt.Println("Which deployment targets should run on this cluster? You can add more later.")
		return c.addTargetPrompt(cc)
	} else {
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
		case 1:
			err = c.removeTargetPrompt(cc)
		}
		return err
	}
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
	err = resources.UpdateClusterConfig(c.kubernetesClient(), cc)
	if err != nil {
		return err
	}
	fmt.Printf("Added target %s to cluster :)\n", val)
	return nil
}

func (c *activeCluster) removeTargetPrompt(cc *v1alpha1.ClusterConfig) error {
	if len(cc.Spec.Targets) == 0 {
		return fmt.Errorf("The cluster doesn't have any targets")
	}
	s := promptui.Select{
		Label: "Select target to remove",
		Items: cc.Spec.Targets,
	}
	idx, val, err := s.Run()
	if err != nil {
		return err
	}

	cc.Spec.Targets = append(cc.Spec.Targets[:idx], cc.Spec.Targets[idx+1:]...)
	err = resources.UpdateClusterConfig(c.kubernetesClient(), cc)
	if err != nil {
		return err
	}
	fmt.Printf("Removed target %s from cluster :)\n", val)
	return nil
}

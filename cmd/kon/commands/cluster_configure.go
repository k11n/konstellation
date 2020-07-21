package commands

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/resources"
)

func (c *activeCluster) configureCluster() error {
	fmt.Printf("Configuring cluster %s\n\n", c.Cluster)
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

		s := utils.NewPromptSelect("Configure action", actions)
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
		Label:     "Target name (enter multiple targets separated by comma)",
		Default:   "production",
		AllowEdit: true,
		Validate: func(val string) error {
			parts := strings.Split(val, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if err := utils.ValidateKubeName(p); err != nil {
					return err
				}
			}
			return nil
		},
	}

	val, err := prompt.Run()
	if err != nil {
		return err
	}

	parts := strings.Split(val, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if funk.Contains(cc.Spec.Targets, p) {
			continue
		}
		cc.Spec.Targets = append(cc.Spec.Targets, p)
	}

	// update linked service accounts before saving targets
	err = reconcileAccounts(c, cc.Spec.Targets)
	if err != nil {
		return errors.Wrap(err, "failed to reconcile linked service accounts")
	}

	_, err = resources.UpdateResource(c.kubernetesClient(), cc, nil, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Added target(s) %s to cluster :)\n", val)
	return nil
}

func (c *activeCluster) removeTargetPrompt(cc *v1alpha1.ClusterConfig) error {
	if len(cc.Spec.Targets) == 0 {
		return fmt.Errorf("The cluster doesn't have any targets")
	}
	s := utils.NewPromptSelect("Select target to remove", cc.Spec.Targets)

	idx, val, err := s.Run()
	if err != nil {
		return err
	}

	cc.Spec.Targets = append(cc.Spec.Targets[:idx], cc.Spec.Targets[idx+1:]...)

	// update linked service accounts before saving targets
	err = reconcileAccounts(c, cc.Spec.Targets)
	if err != nil {
		return errors.Wrap(err, "failed to reconcile linked service accounts")
	}

	_, err = resources.UpdateResource(c.kubernetesClient(), cc, nil, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Removed target %s from cluster :)\n", val)
	return nil
}

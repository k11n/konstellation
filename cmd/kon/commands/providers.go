package commands

import (
	"github.com/davidzhao/konstellation/cmd/kon/providers"
	"github.com/manifoldco/promptui"
)

var (
	CloudAWS        = providers.NewAWSProvider()
	AvailableClouds = []providers.CloudProvider{
		CloudAWS,
	}
)

func CloudProviderByID(id string) providers.CloudProvider {
	for _, c := range AvailableClouds {
		if c.ID() == id {
			return c
		}
	}
	return nil
}

func ChooseCloudPrompt(label string) (providers.CloudProvider, error) {
	if label == "" {
		label = "Choose a cloud provider"
	}
	prompt := promptui.Select{
		Label: label,
		Items: AvailableClouds,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return AvailableClouds[idx], nil
}

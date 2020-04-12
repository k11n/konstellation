package commands

import (
	"fmt"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/providers"
	"github.com/davidzhao/konstellation/cmd/kon/providers/aws"

	"github.com/manifoldco/promptui"
)

var (
	AWSCloud        = aws.NewAWSProvider()
	AvailableClouds = []providers.CloudProvider{
		AWSCloud,
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

func ClusterManagerForCluster(cluster string) (cm providers.ClusterManager, err error) {
	// read config and return the correct manager
	conf := config.GetConfig()
	cl, err := conf.GetClusterLocation(cluster)
	if err == nil {
		return
	}

	cm = NewClusterManager(cl.Cloud, cl.Region)
	if cm == nil {
		err = fmt.Errorf("Could not find manager for cloud %s", cl.Cloud)
	}
	return
}

func ChooseCloudPrompt(label string) (providers.CloudProvider, error) {
	if label == "" {
		label = "Choose a cloud provider"
	}

	clouds := AvailableClouds
	if len(clouds) == 0 {
		fmt.Printf("%s %v\n", label, clouds[0])
		return clouds[0], nil
	}
	prompt := promptui.Select{
		Label: label,
		Items: clouds,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return clouds[idx], nil
}

func ChooseClusterManagerPrompt(label string) (providers.ClusterManager, error) {
	if label == "" {
		label = "Choose a region"
	}

	managers := GetClusterManagers()
	if len(managers) == 1 {
		fmt.Println(managers[0])
		return managers[0], nil
	}
	prompt := promptui.Select{
		Label: label,
		Items: managers,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return managers[idx], nil
}

func GetClusterManagers() []providers.ClusterManager {
	conf := config.GetConfig()
	awsConf := conf.Clouds.AWS
	var items []providers.ClusterManager
	for _, r := range awsConf.Regions {
		items = append(items, aws.NewAWSManager(r))
	}
	return items
}

func NewClusterManager(cloud string, region string) providers.ClusterManager {
	if cloud == "aws" {
		return aws.NewAWSManager(region)
	}
	return nil
}

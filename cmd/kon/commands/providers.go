package commands

import (
	"fmt"

	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/providers"
	"github.com/k11n/konstellation/cmd/kon/providers/aws"
	"github.com/k11n/konstellation/cmd/kon/utils"
)

var (
	AWSCloud        = aws.NewAWSProvider()
	AvailableClouds = []providers.CloudProvider{
		AWSCloud,
	}
)

func GetCloud(name string) providers.CloudProvider {
	for _, cloud := range AvailableClouds {
		if cloud.ID() == name {
			return cloud
		}
	}
	return nil
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
	prompt := utils.NewPromptSelect(label, clouds)

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return clouds[idx], nil
}

func ClusterManagerForCluster(cluster string) (cm providers.ClusterManager, err error) {
	// read config and return the correct manager
	conf := config.GetConfig()
	cl, err := conf.GetClusterLocation(cluster)
	if err != nil {
		return
	}

	cm = NewClusterManager(cl.Cloud, cl.Region)
	if cm == nil {
		err = fmt.Errorf("Could not find manager for cloud %s", cl.Cloud)
	}
	return
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
	prompt := utils.NewPromptSelect(label, managers)

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

func PromptClusterGenerator(cloud providers.CloudProvider, region string) (providers.ClusterConfigGenerator, error) {
	if !cloud.IsSetup() {
		return nil, fmt.Errorf("Provider: %s has not been set up yet", cloud.ID())
	}
	if cloud.ID() == "aws" {
		creds, err := config.GetConfig().Clouds.AWS.GetDefaultCredentials()
		if err != nil {
			return nil, err
		}

		return aws.NewPromptConfigGenerator(region, creds)
	}
	return nil, fmt.Errorf("Unsupported cloud %s", cloud.ID())
}

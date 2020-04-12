package providers

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

type AWSProvider struct {
	id string
}

func NewAWSProvider() *AWSProvider {
	provider := AWSProvider{
		id: "aws",
	}

	return &provider
}

func (a *AWSProvider) ID() string {
	return a.id
}

func (a *AWSProvider) String() string {
	return "AWS"
}

func (a *AWSProvider) IsSetup() bool {
	return config.GetConfig().Clouds.AWS.IsSetup()
}

func (a *AWSProvider) Setup() error {
	conf := config.GetConfig()
	awsConf := &conf.Clouds.AWS
	_, err := awsConf.GetDefaultCredentials()
	if err != nil {
		genericErr := fmt.Errorf("Could not find AWS credentials, run \"aws configure\" to set it")
		// configure aws credentials
		err = cli.RunCommandWithStd("aws", "configure")
		if err != nil {
			return genericErr
		}
		_, err = awsConf.GetDefaultCredentials()
		if err != nil {
			return genericErr
		}
	}

	regions := awsConf.Regions
	if len(regions) == 0 {
		regions = []string{"us-east-1", "us-west-2"}
	}

	// prompt for regions
	validRegions := map[string]bool{}
	resolver := endpoints.DefaultResolver()
	partitions := resolver.(endpoints.EnumPartitions).Partitions()
	for _, p := range partitions {
		if p.ID() == "aws" {
			for id, _ := range p.Regions() {
				validRegions[id] = true
			}
		}
	}
	regionPrompt := promptui.Prompt{
		Label:     "Regions (separate multiple regions with comma)",
		Default:   strings.Join(regions, ","),
		AllowEdit: true,
		Validate: func(s string) error {
			regions := strings.Split(s, ",")
			for _, r := range regions {
				r = strings.TrimSpace(r)
				if !validRegions[r] {
					return fmt.Errorf("Invalid region: %s", r)
				}
			}
			if len(regions) == 0 {
				return fmt.Errorf("no regions provided")
			}
			return nil
		},
	}

	res, err := regionPrompt.Run()
	if err != nil {
		return err
	}

	// validate against actual regions
	awsConf.Regions = []string{}
	for _, r := range strings.Split(res, ",") {
		r = strings.TrimSpace(r)
		if len(r) == 0 {
			continue
		}
		awsConf.Regions = append(awsConf.Regions, r)
	}

	// check if key works
	session, err := sessionForRegion(awsConf.Regions[0])
	if err != nil {
		return errors.Wrapf(err, "AWS credentials are not valid")
	}

	iamSvc := iam.New(session)
	_, err = iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return errors.Wrapf(err, "Couldn't make authenticated calls using provided credentials")
	}

	// TODO: ensure that the permissions we need are accessible

	return conf.Persist()
}

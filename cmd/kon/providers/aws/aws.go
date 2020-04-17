package aws

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

const (
	terraformRegion = "us-west-2"
	terraformBucket = "konstellation"
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
	} else {
		fmt.Println("Found credentials under ~/.aws/credentials. Konstellation will use these credentials to connect to AWS.")
	}

	// prompt for regions
	regions := awsConf.Regions
	if len(regions) == 0 {
		regions = []string{"us-west-2", "us-east-2"}
	}
	validRegions := map[string]bool{}
	resolver := endpoints.DefaultResolver()
	partitions := resolver.(endpoints.EnumPartitions).Partitions()
	for _, p := range partitions {
		if p.ID() == "aws" {
			for id := range p.Regions() {
				validRegions[id] = true
			}
		}
	}
	regionPrompt := promptui.Prompt{
		Label:     "Regions to use (separate multiple regions with comma)",
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
	utils.FixPromptBell(&regionPrompt)

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

	// ask for a bucket to store state
	s3Svc := s3.New(session)
	awsConf.StateS3Bucket, err = a.createStateBucket(s3Svc)
	if err != nil {
		return err
	}

	fmt.Println("AWS has been configured to use with Konstellation!")
	fmt.Println("next, try creating a cluster with `kon cluster create`")
	return conf.Persist()
}

func (a *AWSProvider) createStateBucket(s3Svc *s3.S3) (bucketName string, err error) {
	fmt.Println("Konstellation needs to store configuration in a S3 bucket, enter name of an existing or new bucket.")
	bucketPrompt := promptui.Prompt{
		Label: "Bucket name",
	}
	utils.FixPromptBell(&bucketPrompt)

	bucketPromptFunc := func() (string, error) {
		bn, err := bucketPrompt.Run()
		if err != nil {
			return "", err
		}
		_, err = s3Svc.HeadBucket(&s3.HeadBucketInput{
			Bucket: &bn,
		})
		if err == nil {
			// exists
			return bn, nil
		}
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "NotFound" {
				return bn, nil
			}
		}
		return "", err
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(1)
	}()

	for {
		bucketName, err = bucketPromptFunc()
		if err != nil {
			fmt.Println("Bucket name already in use, please try another name")
		} else {
			break
		}
	}

	_, err = s3Svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		// bucket doesn't exist, try to create it
		bucketPrompt = promptui.Prompt{
			Label:     fmt.Sprintf("Bucket %s doesn't exist, ok to create?", bucketName),
			IsConfirm: true,
		}
		utils.FixPromptBell(&bucketPrompt)
		if _, err = bucketPrompt.Run(); err != nil {
			return
		}
		_, err = s3Svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: &bucketName,
		})
		if err != nil {
			err = errors.Wrap(err, "Could not create bucket")
		}
	}

	return
}

package config

import (
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	"gopkg.in/ini.v1"
)

const (
	defaultProfile = "default"
)

type AWSConfig struct {
	Regions       []string
	StateS3Bucket string
}

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

func (c *AWSConfig) IsSetup() bool {
	if c.StateS3Bucket == "" {
		return false
	}
	creds, err := c.GetDefaultCredentials()
	if err != nil {
		return false
	}
	if creds.AccessKeyID != "" && creds.SecretAccessKey != "" && len(c.Regions) > 0 {
		return true
	}
	return false
}

func (c *AWSConfig) GetDefaultCredentials() (creds *AWSCredentials, err error) {
	return c.GetCredentials(defaultProfile)
}

func (c *AWSConfig) GetCredentials(profile string) (creds *AWSCredentials, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		errors.Wrap(err, "unable to load credentials")
		return
	}

	credsPath := path.Join(home, ".aws", "credentials")
	cfg, err := ini.Load(credsPath)
	if err != nil {
		err = fmt.Errorf("Could not find AWS credentials at ~/.aws/credentials")
		return
	}

	section := cfg.Section(profile)
	if section == nil {
		err = fmt.Errorf("Could not find profile %s", profile)
		return
	}

	creds = &AWSCredentials{}
	key, err := section.GetKey("aws_access_key_id")
	if err != nil {
		return
	}
	creds.AccessKeyID = key.MustString("")
	key, err = section.GetKey("aws_secret_access_key")
	if err != nil {
		return
	}
	creds.SecretAccessKey = key.MustString("")

	return
}

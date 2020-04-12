package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/davidzhao/konstellation/cmd/kon/config"
)

func TestAWSCredentials(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	c := &config.AWSConfig{}
	creds, err := c.GetDefaultCredentials()
	assert.NoError(t, err)
	assert.NotEmpty(t, creds.AccessKeyID)
	assert.NotEmpty(t, creds.SecretAccessKey)
}

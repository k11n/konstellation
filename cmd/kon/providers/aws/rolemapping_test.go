package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRoleMappingList(t *testing.T) {
	// test empty string
	l, err := ParseRoleMappingList("")
	assert.NoError(t, err)
	assert.Nil(t, l)
	assert.False(t, l.HasRole("testrole"))

	// test a role
	content := `- groups:
    - system:bootstrappers
    - system:nodes
  rolearn: arn:aws:iam::807158446417:role/kon-ek2-node-role
  username: system:node:{{EC2PrivateDNSName}}`

	l, err = ParseRoleMappingList(content)
	assert.NoError(t, err)
	assert.NotNil(t, l)
	assert.Len(t, l, 1)
	r := l[0]
	assert.Equal(t, "system:node:{{EC2PrivateDNSName}}", r.Username)
	assert.Equal(t, "arn:aws:iam::807158446417:role/kon-ek2-node-role", r.RoleArn)
	assert.Len(t, r.Groups, 2)
}

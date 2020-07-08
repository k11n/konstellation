package aws

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type IAMRoleMapping struct {
	Groups   []string `yaml:"groups"`
	RoleArn  string   `yaml:"rolearn"`
	Username string   `yaml:"username"`
}

type IAMRoleMappingList []*IAMRoleMapping

func ParseRoleMappingList(content string) (rms IAMRoleMappingList, err error) {
	if content == "" {
		// empty mapping
		return
	}

	// parse yaml
	err = yaml.Unmarshal([]byte(content), &rms)
	if err != nil {
		err = errors.Wrapf(err, "unable to parse role mapping: %s", content)
		return
	}

	return
}

func (l IAMRoleMappingList) HasRole(role string) bool {
	return l.GetRole(role) != nil
}

func (l IAMRoleMappingList) GetRole(role string) *IAMRoleMapping {
	for _, r := range l {
		if r.RoleArn == role {
			return r
		}
	}

	return nil
}

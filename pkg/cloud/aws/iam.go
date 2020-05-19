package aws

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

type IAMService struct {
	IAM *iam.IAM
}

func NewIAMService(s *session.Session) *IAMService {
	return &IAMService{
		IAM: iam.New(s),
	}
}

func (s *IAMService) ListRoles() (roles []*iam.Role, err error) {
	err = s.IAM.ListRolesPages(&iam.ListRolesInput{}, func(output *iam.ListRolesOutput, last bool) bool {
		for _, r := range output.Roles {
			roles = append(roles, r)
		}
		return true
	})
	return
}

/**
 * Lists roles that are not service linked
 */
func (s *IAMService) ListStandardRoles() ([]*iam.Role, error) {
	roleObjs, err := s.ListRoles()
	if err != nil {
		return nil, err
	}
	roles := make([]*iam.Role, 0, len(roleObjs))
	for _, r := range roleObjs {
		if r.Path == nil || strings.HasPrefix(*r.Path, "/aws-service-role") {
			continue
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (s *IAMService) ListEKSServiceRoles() (roles []*iam.Role, err error) {
	stdRoles, err := s.ListStandardRoles()
	if err != nil {
		return
	}
	for _, r := range stdRoles {
		hasPolicies, err := s.hasRequiredPolicies(*r.RoleName, EKSServicePolicyARN, EKSClusterPolicyARN)
		if err != nil {
			return nil, err
		}
		if hasPolicies {
			roles = append(roles, r)
		}
	}
	return
}

func (s *IAMService) hasRequiredPolicies(roleName string, policies ...string) (bool, error) {
	policiesRes, err := s.IAM.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return false, err
	}
	policyMap := map[string]bool{}
	for _, policy := range policies {
		policyMap[policy] = false
	}
	for _, policy := range policiesRes.AttachedPolicies {
		policyMap[*policy.PolicyArn] = true
	}
	hasRequired := true
	for _, hasPolicy := range policyMap {
		if !hasPolicy {
			hasRequired = false
			break
		}
	}
	return hasRequired, nil
}

func (s *IAMService) createRoleWithManagedPolicies(roleName string, trustJSON string, policies ...string) (role *iam.Role, err error) {
	createRes, err := s.IAM.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &roleName,
		AssumeRolePolicyDocument: &trustJSON,
	})
	if err != nil {
		return
	}
	createdRole := createRes.Role

	// apply attached policies
	for _, policy := range policies {
		_, err = s.IAM.AttachRolePolicy(&iam.AttachRolePolicyInput{
			PolicyArn: &policy,
			RoleName:  &roleName,
		})
		if err != nil {
			return
		}
	}

	role = createdRole
	return
}
